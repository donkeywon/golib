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

	"codeberg.org/donkeywon/golib/errs"
	"codeberg.org/donkeywon/golib/util/httpc"
	"codeberg.org/donkeywon/golib/util/ossu"
	"github.com/avast/retry-go/v4"
)

const appendURLSuffix = "?append"

type AppendWriter struct {
	ctx               context.Context
	cfg               Cfg
	cancel            context.CancelFunc
	bufw              *bufio.Writer
	offset            int64
	closeOnce         sync.Once
	needContentLength bool
	isBlob            bool
	blobCreated       bool
	appendFailed      bool
}

func NewAppendWriter(ctx context.Context, cfg Cfg) *AppendWriter {
	cfg.setDefaults()
	w := &AppendWriter{
		cfg:               cfg,
		needContentLength: ossu.NeedContentLength(ctx, cfg.URL),
		isBlob:            ossu.IsAzblob(cfg.URL),
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
		w.appendFailed = true
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
		defer w.cancel()

		if w.bufw != nil {
			err = w.bufw.Flush()
			if err != nil {
				return
			}
		}

		if w.appendFailed {
			return
		}

		err = w.sealBlob()
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
		n, err := io.Copy(w.bufw, r)
		if err != nil {
			w.appendFailed = true
		}
		return n, err
	}

	br := bufio.NewReaderSize(r, 1)
	rr := &readerWrapper{Reader: br}
	for {
		if _, err = br.Peek(1); err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			} else {
				w.appendFailed = true
			}
			break
		}

		lr := io.LimitReader(rr, w.cfg.PartSize)
		err = w.appendPart(httpc.WithBodyReader(lr)) // no retry
		if err != nil {
			w.appendFailed = true
			break
		}
		if w.isBlob {
			w.offset += (w.cfg.PartSize - lr.(*io.LimitedReader).N)
		}
		if rr.eof {
			break
		}
	}

	return rr.nr, err
}

func (w *AppendWriter) sealBlob() error {
	if !w.isBlob || !w.blobCreated {
		return nil
	}
	return retry.Do(
		func() error {
			return ossu.SealAppendBlob(w.ctx, w.cfg.URL, w.cfg.Ak, w.cfg.Sk)
		},
		retry.Attempts(uint(w.cfg.Retry)),
		retry.LastErrorOnly(true),
		retry.Context(w.ctx),
	)
}

func (w *AppendWriter) init() error {
	if !w.isBlob || w.blobCreated {
		return nil
	}
	err := retry.Do(
		func() error { return ossu.CreateAppendBlob(w.ctx, w.cfg.URL, w.cfg.Ak, w.cfg.Sk) },
		retry.Attempts(uint(w.cfg.Retry)),
		retry.LastErrorOnly(true),
		retry.Context(w.ctx),
	)
	if err != nil {
		return errs.Wrap(err, "create append blob failed")
	}
	w.blobCreated = true
	return nil
}

func (w *AppendWriter) addAuth(req *http.Request) error {
	return ossu.Sign(req, w.cfg.Ak, w.cfg.Sk, w.cfg.Region)
}

func (w *AppendWriter) retryAppendPart(p []byte) error {
	return retry.Do(
		func() error {
			return w.appendPart(httpc.WithBody(p))
		},
		retry.Attempts(uint(w.cfg.Retry)),
		retry.LastErrorOnly(true),
		retry.Context(w.ctx),
	)
}

func (w *AppendWriter) appendPart(opts ...httpc.Option) error {
	var (
		url            string
		respBody       = bytes.NewBuffer(nil)
		respStatusCode int
		err            error
		allOpts        = make([]httpc.Option, 0, len(opts)+3)
	)

	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, httpc.ToWriter(respBody, nil))
	if w.isBlob {
		url = w.cfg.URL + "?comp=appendblock"
		allOpts = append(allOpts,
			httpc.WithHeaders(ossu.HeaderAzblobAppendPositionHeader, strconv.FormatInt(w.offset, 10)),
			httpc.CheckStatusCode(respBody, &respStatusCode, http.StatusCreated),
		)
	} else {
		url = w.cfg.URL + appendURLSuffix + fmt.Sprintf("&position=%d", w.offset)
		allOpts = append(allOpts, httpc.CheckStatusCode(respBody, &respStatusCode, http.StatusOK))
	}

	var header http.Header
	allOpts = append(allOpts, httpc.ReqOptionFunc(w.addAuth), httpc.RespOptionFunc(func(resp *http.Response) error {
		header = resp.Header
		return nil
	}))
	err = w.append(url, allOpts...)
	if err != nil {
		return errs.Wrapf(err, "append failed with max retry, offset: %d, resp status code: %d, resp body: %s", w.offset, respStatusCode, respBody.String())
	}

	if w.isBlob {
		return nil
	}

	pos, exists, err := ossu.GetNextPositionFromResponseHeader(header)
	if err != nil {
		return errs.Wrap(err, "failed to get next position from response")
	}
	if !exists {
		return errs.Errorf("next position not exists in response")
	}
	w.offset = int64(pos)
	return nil
}

func (w *AppendWriter) append(url string, opts ...httpc.Option) error {
	if w.isBlob {
		return httpc.PutTimeout(w.ctx, w.cfg.Timeout, url, opts...)
	}
	return httpc.PostTimeout(w.ctx, w.cfg.Timeout, url, opts...)
}

func (w *AppendWriter) Offset() int64 {
	return w.offset
}
