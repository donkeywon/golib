package oss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/oss"
)

var contentRangeReg = regexp.MustCompile(`(\d+)-(\d+)/(\d+)`)

const contentRangeSubMatchLen = 4

type Reader struct {
	*Cfg
	Pos int

	eof       bool
	closed    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	headOnce  sync.Once
	headErr   error
	total     int
}

func NewReader() *Reader {
	r := &Reader{
		Cfg:    NewCfg(),
		closed: make(chan struct{}),
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	return r
}

func (r *Reader) Read(p []byte) (int, error) {
	select {
	case <-r.closed:
		return 0, ErrAlreadyClosed
	default:
	}

	if r.eof {
		return 0, io.EOF
	}

	r.headOnce.Do(func() {
		resp, err := retry.DoWithData(
			func() (*http.Response, error) {
				return oss.Head(r.ctx, time.Second*time.Duration(r.Cfg.Timeout), r.URL, r.Ak, r.Sk, r.Region)
			},
			retry.Attempts(uint(r.Retry)),
		)
		if err != nil {
			r.headErr = errs.Wrap(err, "head oss failed")
			return
		}
		r.total, err = strconv.Atoi(resp.Header.Get("Content-Length"))
		if err != nil {
			r.headErr = errs.Errorf("oss head response Content-Length is invalid: %+v", resp.Header)
			return
		}
	})

	if r.headErr != nil {
		return 0, r.headErr
	}

	if len(p) == 0 {
		return 0, errs.New("dst buf is empty")
	}

	if r.Retry <= 0 {
		r.Retry = 3
	}
	nr, err := r.retryRead(r.Pos, p)
	r.Pos += nr
	if err != nil {
		return nr, errs.Wrap(err, "retry read oss object failed")
	}

	return nr, nil
}

func (r *Reader) Close() error {
	r.closeOnce.Do(func() {
		close(r.closed)
		r.cancel()
	})
	return nil
}

func (r *Reader) addAuth(req *http.Request) error {
	if r.Ak == "" && r.Sk == "" {
		return nil
	}
	return oss.Sign(req, r.Ak, r.Sk, r.Region)
}

func (r *Reader) retryRead(start int, p []byte) (int, error) {
	nr, err := retry.DoWithData(
		func() (int, error) {
			return r.readPart(start, p)
		},
		retry.Attempts(uint(r.Retry)),
		retry.RetryIf(func(err error) bool {
			return err != nil && !errors.Is(err, io.EOF)
		}),
	)

	if err != nil {
		return 0, errs.Wrapf(err, "read oss object fail with max retry: %d", r.Retry)
	}
	return nr, nil
}

func (r *Reader) readPart(start int, p []byte) (int, error) {
	var nr int

	end := len(p) + start - 1
	if end >= r.total-1 {
		end = r.total - 1
	}
	reqRange := fmt.Sprintf("bytes=%d-%d", start, end)

	resp, err := httpc.Get(r.ctx, time.Second*time.Duration(r.Timeout), r.URL,
		httpc.WithHeaders("Range", reqRange),
		httpc.ReqOptionFunc(r.addAuth),
		httpc.ToBytes(&nr, p),
	)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if err != nil {
		return nr, errs.Wrap(err, "get oss object part failed")
	}

	contentRange := resp.Header.Get("Content-Range")
	contentLength := resp.Header.Get("Content-Length")
	if contentRange == "" || contentLength == "" {
		return nr, errs.Errorf("response header Content-Length or Content-Range is empty, header: %+v", resp.Header)
	}

	contentLengthN, err := strconv.Atoi(contentLength)
	if err != nil {
		return nr, errs.Wrapf(err, "resp header Content-Length is not number: %s", contentLength)
	}

	contentRangeS := contentRangeReg.FindStringSubmatch(contentRange)
	if len(contentRangeS) != contentRangeSubMatchLen {
		return nr, errs.Errorf("resp header Content-Range is not valid: %s", contentRange)
	}

	// respContentStart, _ := strconv.Atoi(contentRangeS[1])
	respContentEnd, _ := strconv.Atoi(contentRangeS[2])
	contentTotal, _ := strconv.Atoi(contentRangeS[3])

	if contentLengthN == 0 && respContentEnd != contentTotal-1 {
		return nr, errs.Errorf("read not at eof but length is 0, Content-Range: %s", contentRange)
	}

	if respContentEnd == contentTotal-1 {
		r.eof = true
		return nr, nil
	}

	return nr, nil
}
