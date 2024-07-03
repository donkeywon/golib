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
		Cfg:    NewCfg(),
		closed: make(chan struct{}),
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	return w
}

func (w *AppendWriter) Write(p []byte) (int, error) {
	select {
	case <-w.closed:
		return 0, errs.ErrWriteToClosedWriter
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
			retry.Delay(time.Second),
		)
		if err != nil {
			return 0, errs.Wrap(err, "delete oss object fail")
		}
		w.deleted = true
	}

	if oss.IsAzblob(w.URL) && !w.created {
		err = retry.Do(
			func() error {
				return oss.CreateAppendBlob(w.ctx, w.URL, w.Ak, w.Sk)
			},
			retry.Attempts(uint(w.Retry)),
			retry.Delay(time.Second),
		)
		if err != nil {
			return 0, errs.Wrap(err, "create append blob fail")
		}
		w.created = true
	}

	resp, err := w.retryAppend(p)
	if err != nil {
		return 0, errs.Wrapf(err, "append to oss object fail, resp: %s", string(resp))
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
	ctx, cancel := context.WithTimeout(w.ctx, time.Second*time.Duration(w.Timeout))
	defer cancel()
	return oss.Delete(ctx, w.URL, w.Ak, w.Sk, w.Region)
}

func (w *AppendWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.Ak, w.Sk, w.Region)
}

func (w *AppendWriter) retryAppend(payload []byte) ([]byte, error) {
	var (
		respBody  []byte
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
		retry.Delay(time.Second),
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
		return respBody, errs.Wrap(retryErrs, "append failed with max retry")
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
			return respBody, errs.Wrapf(err, "get next position from response fail, header: %+v", resp.Header)
		}
	}

	if !exists {
		return respBody, errs.Errorf("next position not exists in response: %+v", resp.Header)
	}
	if w.Pos+len(payload) > pos {
		return respBody, errs.Errorf("not all body been written, next: %d, cur: %d, body len: %d", pos, w.Pos, len(payload))
	}
	w.Pos = pos

	return respBody, nil
}

func (w *AppendWriter) doAppend(body []byte) ([]byte, *http.Response, error) {
	var (
		req *http.Request
		err error
	)
	if oss.IsAzblob(w.URL) {
		req, err = http.NewRequestWithContext(w.ctx, http.MethodPut, w.URL+"?comp=appendblock", bytes.NewReader(body))
		if req != nil {
			req.Header.Set(oss.HeaderAzblobAppendPositionHeader, strconv.Itoa(w.Pos))
		}
	} else {
		req, err = http.NewRequestWithContext(w.ctx, http.MethodPost,
			w.URL+appendURLSuffix+fmt.Sprintf("&position=%d", w.Pos), bytes.NewReader(body))
	}

	if err != nil {
		return nil, nil, errs.Wrap(err, "new append request fail")
	}

	err = w.addAuth(req)
	if err != nil {
		return nil, nil, errs.Wrap(err, "sign oss req fail")
	}

	req.ContentLength = int64(len(body))

	body, resp, err := httpc.DoBody(req)
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, nil, errs.Wrap(err, "do http request fail")
	}

	if oss.IsAzblob(w.URL) {
		if resp.StatusCode != http.StatusCreated {
			return body, resp, errs.Errorf("http resp status code is not created: %d, body: %s", resp.StatusCode, string(body))
		}
	} else {
		if resp.StatusCode != http.StatusOK {
			return body, resp, errs.Errorf("http resp status code is not ok: %d, body: %s", resp.StatusCode, string(body))
		}
	}

	return body, resp, nil
}
