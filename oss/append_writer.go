package oss

import (
	"bufio"
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

const appendURLSuffix = "?append"

type AppendWriter struct {
	ctx               context.Context
	cfg               *Cfg
	cancel            context.CancelFunc
	bufw              *bufio.Writer
	timeout           time.Duration
	offset            int64
	closeOnce         sync.Once
	needContentLength bool
	isBlob            bool
	blobCreated       bool
}

func NewAppendWriter(ctx context.Context, cfg *Cfg) *AppendWriter {
	cfg.setDefaults()
	w := &AppendWriter{
		cfg:               cfg,
		timeout:           time.Second * time.Duration(cfg.Timeout),
		needContentLength: oss.NeedContentLength(cfg.URL),
		isBlob:            oss.IsAzblob(cfg.URL),
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.offset = cfg.Offset
	return w
}

func (w *AppendWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	err := w.init()
	if err != nil {
		return 0, err
	}

	err = w.retryAppendPart(p)
	if err != nil {
		return 0, errs.Wrap(err, "append part failed")
	}

	if w.isBlob {
		w.offset += int64(len(p))
	}

	return len(p), nil
}

func (w *AppendWriter) Close() error {
	var err error
	w.closeOnce.Do(func() {
		if w.bufw != nil {
			err = w.bufw.Flush()
		}
		err = errors.Join(err, w.sealBlob())
		w.cancel()
	})
	return err
}

type apwWithoutReadFrom struct {
	noReadFrom
	*AppendWriter
}

func (w *AppendWriter) ReadFrom(r io.Reader) (int64, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
	}

	err := w.init()
	if err != nil {
		return 0, err
	}

	if w.needContentLength {
		w.bufw = bufio.NewWriterSize(apwWithoutReadFrom{AppendWriter: w}, int(w.cfg.PartSize))
		return io.Copy(w.bufw, r)
	}

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

func (w *AppendWriter) sealBlob() error {
	if !w.isBlob {
		return nil
	}
	return retry.Do(
		func() error {
			return oss.SealAppendBlob(w.ctx, w.cfg.URL, w.cfg.Ak, w.cfg.Sk)
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
		retry.LastErrorOnly(true),
	)
}

func (w *AppendWriter) init() error {
	if !w.isBlob || w.blobCreated {
		return nil
	}
	err := retry.Do(
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
		retry.LastErrorOnly(true),
	)
	if err != nil {
		return errs.Wrap(err, "create append blob failed")
	}
	w.blobCreated = true
	return nil
}

func (w *AppendWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.cfg.Ak, w.cfg.Sk, w.cfg.Region)
}

func (w *AppendWriter) retryAppendPart(p []byte) error {
	return retry.Do(
		func() error {
			return w.appendPart(httpc.WithBody(p))
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
		retry.LastErrorOnly(true),
	)
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
			httpc.WithHeaders(oss.HeaderAzblobAppendPositionHeader, strconv.FormatInt(w.offset, 10)),
			httpc.CheckStatusCode(http.StatusCreated),
		)
	} else {
		url = w.cfg.URL + appendURLSuffix + fmt.Sprintf("&position=%d", w.offset)
		allOpts = append(allOpts, httpc.CheckStatusCode(http.StatusOK))
	}

	allOpts = append(allOpts, httpc.ReqOptionFunc(w.addAuth))

	resp, err = w.append(url, allOpts...)
	if err != nil {
		return errs.Wrapf(err, "append failed with max retry, offset: %d, respStatus: %s, respBody: %s", w.offset, respStatus, respBody.String())
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
	w.offset = int64(pos)
	return nil
}

func (w *AppendWriter) append(url string, opts ...httpc.Option) (*http.Response, error) {
	if w.isBlob {
		return httpc.Put(w.ctx, 0, url, opts...)
	}
	return httpc.Post(w.ctx, 0, url, opts...)
}

func (w *AppendWriter) Offset() int64 {
	return w.offset
}
