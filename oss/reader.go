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
	"github.com/donkeywon/golib/common"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/oss"
)

var contentRangeReg = regexp.MustCompile(`(\d+)-(\d+)/(\d+)`)

const contentRangeSubMatchLen = 4

type Reader struct {
	*Cfg
	Pos int

	eof    bool
	closed chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

func NewReader() *Reader {
	r := &Reader{
		closed: make(chan struct{}),
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	return r
}

func (r *Reader) Read(p []byte) (int, error) {
	select {
	case <-r.closed:
		return 0, common.ErrReadFromClosedReader
	default:
	}

	if r.eof {
		return 0, io.EOF
	}

	if len(p) == 0 {
		return 0, errs.New("dst buf is empty")
	}

	if r.Retry <= 0 {
		r.Retry = 3
	}
	nr, err := r.retryRead(r.Pos, p)
	r.Pos += nr
	if errors.Is(err, io.EOF) {
		r.eof = true
		return nr, err
	}
	if err != nil {
		return nr, errs.Wrap(err, "retry read oss object fail")
	}

	return nr, nil
}

func (r *Reader) Close() error {
	r.once.Do(func() {
		close(r.closed)
		r.cancel()
	})
	return nil
}

func (r *Reader) addAuth(req *http.Request) error {
	return oss.Sign(req, r.Ak, r.Sk, r.Region)
}

func (r *Reader) retryRead(start int, p []byte) (int, error) {
	nr, err := retry.DoWithData(
		func() (int, error) {
			return r.read(start, p)
		},
		retry.Attempts(uint(r.Retry)),
		retry.RetryIf(func(err error) bool {
			return err != nil && !errors.Is(err, io.EOF)
		}),
		retry.Delay(time.Second),
	)

	if err != nil {
		return 0, errs.Wrapf(err, "read oss object fail with max retry: %d", r.Retry)
	}
	return nr, nil
}

func (r *Reader) read(start int, p []byte) (int, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return 0, errs.Wrap(err, "new read request fail")
	}

	reqRange := fmt.Sprintf("bytes=%d-%d", start, len(p)+start-1)
	req.Header.Set("Range", reqRange)

	err = r.addAuth(req)
	if err != nil {
		return 0, errs.Wrap(err, "sign oss req fail")
	}

	resp, err := httpc.DoTimeout(req, time.Second*time.Duration(r.Timeout))
	if err != nil {
		return 0, errs.Wrap(err, "do http request fail")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, errs.Errorf("http resp status is not ok and partial content: %d", resp.StatusCode)
	}

	nr, err := io.ReadFull(resp.Body, p)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		err = nil
	}
	if err != nil {
		return 0, errs.Wrap(err, "read resp body fail")
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
		return nr, io.EOF
	}

	return nr, nil
}
