package oss

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/oss"
)

var ErrAppendNotSupported = errors.New("append not supported")

const appendURLSuffix = "?append"

type AppendWriter struct {
	cfg *Cfg

	timeout   time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	supportAppend bool
	isBlob        bool
	blobCreated   bool
}

func NewAppendWriter(ctx context.Context, cfg *Cfg) *AppendWriter {
	cfg.setDefaults()
	w := &AppendWriter{
		cfg:     cfg,
		timeout: time.Second * time.Duration(cfg.Timeout),
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.isBlob = oss.IsAzblob(cfg.URL)
	w.supportAppend = oss.IsSupportAppend(cfg.URL)
	return w
}

func (w *AppendWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, ErrAlreadyClosed
	default:
	}

	if !w.supportAppend {
		return 0, ErrAppendNotSupported
	}

	if len(p) == 0 {
		return 0, nil
	}

	var err error

	if w.isBlob && !w.blobCreated {
		err = retry.Do(
			func() error { return oss.CreateAppendBlob(w.ctx, w.cfg.URL, w.cfg.Ak, w.cfg.Sk) },
			retry.Attempts(uint(w.cfg.Retry)),
			retry.RetryIf(func(err error) bool {
				select {
				case <-w.ctx.Done():
					return false
				default:
					return err != nil
				}
			}),
		)
		if err != nil {
			return 0, errs.Wrap(err, "create append blob failed")
		}
		w.blobCreated = true
	}

	err = w.appendPart(httpc.WithBody(p))
	if err != nil {
		return 0, errs.Wrap(err, "append part failed")
	}

	return len(p), nil
}

func (w *AppendWriter) Close() error {
	w.closeOnce.Do(func() {
		w.cancel()
	})
	return nil
}

func (w *AppendWriter) ReadFrom(r io.Reader) (int64, error) {
	select {
	case <-w.ctx.Done():
		return 0, ErrAlreadyClosed
	default:
	}

	if !w.supportAppend {
		return 0, ErrAppendNotSupported
	}

	var err error
	rr := &readerWrapper{Reader: r}
	for {
		lr := io.LimitReader(rr, w.cfg.PartSize)
		err = w.appendPart(httpc.WithBodyReader(lr))
		if err != nil {
			break
		}
		if rr.eof {
			break
		}
	}

	return rr.nr, err
}

func (w *AppendWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.cfg.Ak, w.cfg.Sk, w.cfg.Region)
}

func (w *AppendWriter) appendPart(opts ...httpc.Option) error {
	var (
		url        string
		respBody   = bytes.NewBuffer(nil)
		respStatus string
		resp       *http.Response
		err        error
		allOpts    = make([]httpc.Option, 0, len(opts))
	)

	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, httpc.ToStatus(&respStatus), httpc.ToBytesBuffer(respBody))
	if w.isBlob {
		url = w.cfg.URL + "?comp=appendblock"
		allOpts = append(allOpts,
			httpc.WithHeaders(oss.HeaderAzblobAppendPositionHeader, strconv.FormatInt(w.cfg.Offset, 10)),
			httpc.CheckStatusCode(http.StatusCreated),
		)
	} else {
		url = w.cfg.URL + appendURLSuffix + fmt.Sprintf("&position=%d", w.cfg.Offset)
		allOpts = append(allOpts, httpc.CheckStatusCode(http.StatusOK))
	}

	allOpts = append(allOpts, httpc.ReqOptionFunc(w.addAuth))

	resp, err = retry.DoWithData(
		func() (*http.Response, error) {
			return w.append(url, allOpts...)
		},
		retry.Attempts(uint(w.cfg.Retry)),
		retry.RetryIf(func(err error) bool {
			select {
			case <-w.ctx.Done():
				return false
			default:
				return err != nil
			}
		}),
	)
	if err != nil {
		return errs.Wrapf(err, "append failed with max retry, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}

	if w.isBlob {
		return nil
	}

	pos, exists, err := oss.GetNextPositionFromResponse(resp)
	if err != nil {
		return errs.Wrap(err, "failed to get next position from response")
	}
	if !exists {
		return errs.Errorf("next position not exists in response")
	}
	w.cfg.Offset = int64(pos)
	return nil
}

func (w *AppendWriter) append(url string, opts ...httpc.Option) (*http.Response, error) {
	if w.isBlob {
		return httpc.Put(w.ctx, w.timeout, url, opts...)
	}
	return httpc.Post(w.ctx, w.timeout, url, opts...)
}
