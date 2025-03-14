package oss

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/oss"
)

const appendURLSuffix = "?append"

type AppendWriter struct {
	*Cfg
	Pos           int
	NoDeleteFirst bool

	closed chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	deleted bool
	created bool
}

func NewAppendWriter() *AppendWriter {
	w := &AppendWriter{
		Cfg:    &Cfg{},
		closed: make(chan struct{}),
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	return w
}

func (w *AppendWriter) Write(p []byte) (int, error) {
	select {
	case <-w.closed:
		return 0, ErrAlreadyClosed
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	if w.Retry <= 0 {
		w.Retry = 3
	}

	var err error

	if !w.NoDeleteFirst && !w.deleted {
		err = retry.Do(
			func() error {
				return w.delete()
			},
			retry.Attempts(uint(w.Retry)),
		)
		if err != nil {
			return 0, errs.Wrap(err, "delete oss object failed")
		}
		w.deleted = true
	}

	if oss.IsAzblob(w.URL) && !w.created {
		err = retry.Do(
			func() error { return oss.CreateAppendBlob(w.ctx, w.URL, w.Ak, w.Sk) },
			retry.Attempts(uint(w.Retry)),
		)
		if err != nil {
			return 0, errs.Wrap(err, "create append blob failed")
		}
		w.created = true
	}

	respBody, resp, err := w.retryAppend(p)
	if err != nil {
		return 0, errs.Wrapf(err, "append to oss object fail, resp: %v, resp body: %s", resp, respBody.String())
	}

	return len(p), nil
}

func (w *AppendWriter) Close() error {
	w.once.Do(func() {
		close(w.closed)
		w.cancel()
	})
	return nil
}

func (w *AppendWriter) delete() error {
	return oss.Delete(w.ctx, time.Second*time.Duration(w.Timeout), w.URL, w.Ak, w.Sk, w.Region)
}

func (w *AppendWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.Ak, w.Sk, w.Region)
}

func (w *AppendWriter) retryAppend(payload []byte) (*bytes.Buffer, *http.Response, error) {
	var (
		respBody  *bytes.Buffer
		resp      *http.Response
		retryErrs error
	)

	err := retry.Do(
		func() error {
			var e error
			respBody, resp, e = w.doAppend(payload)
			retryErrs = errors.Join(retryErrs, e)
			return e
		},
		retry.Attempts(uint(w.Retry)),
		retry.RetryIf(func(_ error) bool {
			select {
			case <-w.closed:
				return false
			default:
				return true
			}
		}),
	)

	if err != nil {
		return respBody, resp, errs.Wrap(retryErrs, "append failed with max retry")
	}

	var (
		pos    int
		exists bool
	)
	if oss.IsAzblob(w.URL) {
		exists = true
		pos = w.Pos + len(payload)
	} else {
		pos, exists, err = oss.GetNextPositionFromResponse(resp)
		if err != nil {
			return respBody, resp, errs.Wrapf(err, "get next position from response failed")
		}
	}

	if !exists {
		return respBody, resp, errs.Errorf("next position not exists in response")
	}
	if w.Pos+len(payload) > pos {
		return respBody, resp, errs.Errorf("not all body been written, next: %d, cur: %d, body len: %d", pos, w.Pos, len(payload))
	}
	w.Pos = pos

	return respBody, resp, nil
}

func (w *AppendWriter) doAppend(body []byte) (*bytes.Buffer, *http.Response, error) {
	var (
		resp     *http.Response
		respBody = bytes.NewBuffer(nil)
		err      error
	)

	if oss.IsAzblob(w.URL) {
		resp, err = httpc.Put(w.ctx, time.Second*time.Duration(w.Timeout), w.URL+"?comp=appendblock",
			httpc.WithBody(body),
			httpc.WithHeaders(oss.HeaderAzblobAppendPositionHeader, strconv.Itoa(w.Pos)),
			httpc.ReqOptionFunc(w.addAuth),
			httpc.CheckStatusCode(http.StatusCreated),
			httpc.ToBytesBuffer(respBody))
	} else {
		url := w.URL + appendURLSuffix + fmt.Sprintf("&position=%d", w.Pos)
		resp, err = httpc.Put(w.ctx, time.Second*time.Duration(w.Timeout), url,
			httpc.WithBody(body),
			httpc.ReqOptionFunc(w.addAuth),
			httpc.CheckStatusCode(http.StatusOK),
			httpc.ToBytesBuffer(respBody))
	}

	if err == nil || errors.Is(err, context.Canceled) {
		return respBody, resp, nil
	}
	return respBody, resp, errs.Wrap(err, "do http response failed")
}
