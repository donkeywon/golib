package oss

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/donkeywon/golib/util/iou"
	"github.com/donkeywon/golib/util/oss"
)

type InitiateMultipartUploadResult struct {
	UploadID string `xml:"UploadId"`
}

type CompleteMultipartUpload struct {
	Parts []*Part `xml:"Part"`
}

type Part struct {
	ETag       string `xml:"ETag"`
	PartNumber int    `xml:"PartNumber"`
}

type BlockList struct {
	Latest []string `xml:"Latest"`
}

type MultiPartWriter struct {
	ctx               context.Context
	uploadErr         error
	parallelChan      chan *uploadPartReq
	bufChan           chan []byte
	bufw              *bufio.Writer
	cancel            context.CancelFunc
	cfg               *Cfg
	uploadID          string
	parts             []*Part
	blockList         []string
	parallelResult    []*uploadPartResult
	parallelErrs      []error
	parallelWg        sync.WaitGroup
	curPartNo         int
	timeout           time.Duration
	parallelChanOnce  sync.Once
	closeOnce         sync.Once
	bufChanOnce       sync.Once
	mu                sync.Mutex
	initialized       bool
	needContentLength bool
	isBlob            bool
}

func NewMultiPartWriter(ctx context.Context, cfg *Cfg) *MultiPartWriter {
	cfg.setDefaults()
	w := &MultiPartWriter{
		cfg:               cfg,
		timeout:           time.Second * time.Duration(cfg.Timeout),
		isBlob:            oss.IsAzblob(cfg.URL),
		needContentLength: oss.NeedContentLength(cfg.URL),
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	return w
}

type noReadFrom struct{}

func (noReadFrom) ReadFrom(r io.Reader) (n int64, err error) { panic("can't happen") }

type mpwWithoutReadFrom struct {
	noReadFrom
	*MultiPartWriter
}

func (w *MultiPartWriter) ReadFrom(r io.Reader) (int64, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
	}

	err := w.init()
	if err != nil {
		return 0, err
	}

	if w.needContentLength && w.cfg.Parallel == 1 {
		w.bufw = bufio.NewWriterSize(mpwWithoutReadFrom{MultiPartWriter: w}, int(w.cfg.PartSize))
		return io.Copy(w.bufw, r)
	}

	if w.cfg.Parallel > 1 {
		var (
			total int64
			n     int
			b     []byte
		)
		for {
			select {
			case b = <-w.bufChan:
			default:
				b = make([]byte, w.cfg.PartSize)
			}
			b = b[:cap(b)]
			n, err = iou.ReadFill(b, r)
			total += int64(n)
			if n > 0 {
				b = b[:n]
				w.parallelChan <- &uploadPartReq{partNo: w.curPartNo, b: b}
				w.curPartNo++
			}
			if err != nil {
				break
			}
		}

		w.closeParallelChan()
		w.parallelWg.Wait()
		w.closeBufChan()
		if err == io.EOF {
			err = nil
		}

		return total, errors.Join(err, w.parallelErr())
	}

	rr := &readerWrapper{Reader: r}
	for {
		lr := io.LimitReader(rr, w.cfg.PartSize)
		result := w.uploadPart(w.curPartNo, httpc.WithBodyReader(lr))
		w.curPartNo++
		err = result.err
		if err != nil {
			w.uploadErr = err
			break
		}
		w.handleUploadPartResult(result)
		if rr.eof {
			break
		}
	}

	return rr.nr, err
}

type readerWrapper struct {
	io.Reader
	eof bool
	nr  int64
}

func (r *readerWrapper) Read(p []byte) (int, error) {
	nr, err := r.Reader.Read(p)
	r.nr += int64(nr)
	r.eof = err == io.EOF
	return nr, err
}

type uploadPartReq struct {
	b      []byte
	partNo int
}

type uploadPartResult struct {
	err   error
	part  *Part
	block string

	partNo int
}

func (w *MultiPartWriter) Write(p []byte) (int, error) {
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

	var b []byte
	if w.cfg.Parallel > 1 {
		err = w.parallelErr()
		if err != nil {
			return 0, err
		}

		select {
		case b = <-w.bufChan:
		default:
			b = make([]byte, w.cfg.PartSize)
		}
		b = b[:cap(b)]
		nc := copy(b, p)
		b = b[:nc]
		w.parallelChan <- &uploadPartReq{partNo: w.curPartNo, b: b}
		w.curPartNo++
		return len(p), nil
	}

	r := w.uploadPart(w.curPartNo, httpc.WithBody(p))
	w.curPartNo++
	if r.err != nil {
		if w.uploadErr == nil {
			w.uploadErr = r.err
		}
		return 0, errs.Wrap(r.err, "upload part failed")
	}
	w.handleUploadPartResult(r)

	return len(p), nil
}

func (w *MultiPartWriter) handleParallelResult(r *uploadPartResult) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if r.err != nil {
		w.parallelErrs = append(w.parallelErrs, r.err)
		return
	}

	w.parallelResult = append(w.parallelResult, r)
}

func (w *MultiPartWriter) handleUploadPartResult(r *uploadPartResult) {
	if w.isBlob {
		w.blockList = append(w.blockList, r.block)
	} else {
		w.parts = append(w.parts, r.part)
	}
}

func (w *MultiPartWriter) parallelErr() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return errors.Join(w.parallelErrs...)
}

func (w *MultiPartWriter) Close() error {
	var err error
	if w.bufw != nil {
		err = w.bufw.Flush()
	}
	w.closeOnce.Do(func() {
		w.closeParallelChan()
		w.parallelWg.Wait()
		w.closeBufChan()

		w.cancel()
		if w.uploadErr == nil {
			err = errors.Join(err, w.complete())
		} else {
			err = errors.Join(err, w.abort())
		}
	})
	return err
}

func (w *MultiPartWriter) closeParallelChan() {
	w.parallelChanOnce.Do(func() {
		if w.parallelChan != nil {
			close(w.parallelChan)
		}
	})
}

func (w *MultiPartWriter) closeBufChan() {
	w.bufChanOnce.Do(func() {
		if w.bufChan != nil {
			close(w.bufChan)
		}
	})
}

func (w *MultiPartWriter) init() error {
	if w.initialized {
		return nil
	}

	w.needContentLength = oss.NeedContentLength(w.cfg.URL)

	var err error
	if w.isBlob {
		w.initialized = true
		goto PARALLEL
	}

	w.uploadID, err = w.initMultiPart()
	if err != nil {
		return errs.Wrap(err, "init multi part failed")
	}
	w.initialized = true
	w.curPartNo = 1

PARALLEL:
	if w.cfg.Parallel > 1 {
		w.bufChan = make(chan []byte)
		w.parallelWg.Add(w.cfg.Parallel)
		w.parallelChan = make(chan *uploadPartReq)
		for range w.cfg.Parallel {
			go w.uploadWorker()
		}
	}

	return nil
}

func (w *MultiPartWriter) uploadWorker() {
	defer w.parallelWg.Done()
	for {
		select {
		case <-w.ctx.Done():
			return
		case req, ok := <-w.parallelChan:
			if !ok {
				return
			}
			err := w.parallelErr()
			if err != nil {
				w.bufChan <- req.b
				continue
			}

			r := w.uploadPart(req.partNo, httpc.WithBody(req.b))
			w.bufChan <- req.b
			w.handleParallelResult(r)
		}
	}
}

func (w *MultiPartWriter) abort() error {
	if w.isBlob {
		return nil
	}

	var (
		respBody   = bytes.NewBuffer(nil)
		respStatus string
	)
	_, err := retry.DoWithData(
		func() (*http.Response, error) {
			respBody.Reset()
			return httpc.Delete(context.TODO(), w.timeout, w.cfg.URL+"?uploadId="+w.uploadID,
				httpc.ReqOptionFunc(w.addAuth),
				httpc.ToStatus(&respStatus),
				httpc.ToBytesBuffer(respBody),
				httpc.CheckStatusCode(http.StatusNoContent),
			)
		},
		retry.Attempts(uint(w.cfg.Retry)),
		retry.LastErrorOnly(true),
	)

	if err != nil {
		return errs.Wrapf(err, "abort multipart fail, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}
	return nil
}

func (w *MultiPartWriter) complete() error {
	if w.cfg.Parallel > 1 {
		slices.SortFunc(w.parallelResult, func(a, b *uploadPartResult) int {
			return a.partNo - b.partNo
		})
		if w.isBlob {
			w.blockList = make([]string, len(w.parallelResult))
			for i, r := range w.parallelResult {
				w.blockList[i] = r.block
			}
		} else {
			w.parts = make([]*Part, len(w.parallelResult))
			for i, r := range w.parallelResult {
				w.parts[i] = r.part
			}
		}
	}

	if len(w.blockList) == 0 && len(w.parts) == 0 {
		return nil
	}

	var (
		url         string
		err         error
		body        any
		checkStatus int
		respStatus  string
		respBody    = bytes.NewBuffer(nil)
		contentType string
		method      string
	)

	if w.isBlob {
		url = w.cfg.URL + "?comp=blocklist"
		checkStatus = http.StatusCreated
		body = &BlockList{Latest: w.blockList}
		contentType = httpu.MIMEPlainUTF8
		method = http.MethodPut
	} else {
		url = w.cfg.URL + "?uploadId=" + w.uploadID
		checkStatus = http.StatusOK
		body = &CompleteMultipartUpload{Parts: w.parts}
		contentType = httpu.MIMEXML
		method = http.MethodPost
	}

	_, err = retry.DoWithData(
		func() (*http.Response, error) {
			respBody.Reset()
			return httpc.Do(context.TODO(), w.timeout, method, url,
				httpc.WithBodyMarshal(body, contentType, xml.Marshal),
				httpc.ReqOptionFunc(w.addAuth),
				httpc.ToStatus(&respStatus),
				httpc.ToBytesBuffer(respBody),
				httpc.CheckStatusCode(checkStatus),
			)
		},
		retry.LastErrorOnly(true),
		retry.Attempts(uint(w.cfg.Retry)),
	)

	if err != nil {
		return errs.Wrapf(err, "retry do complete multipart request fail, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}
	return nil
}

func (w *MultiPartWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.cfg.Ak, w.cfg.Sk, w.cfg.Region)
}

func (w *MultiPartWriter) initMultiPart() (string, error) {
	var (
		respStatus string
		respBody   = bytes.NewBuffer(nil)
		err        error
	)
	result := &InitiateMultipartUploadResult{}
	err = retry.Do(
		func() error {
			_, err = httpc.Post(w.ctx, w.timeout, w.cfg.URL+"?uploads",
				httpc.ReqOptionFunc(w.addAuth),
				httpc.ToStatus(&respStatus),
				httpc.ToBytesBuffer(respBody),
				httpc.CheckStatusCode(http.StatusOK),
			)
			return err
		},
		retry.LastErrorOnly(true),
		retry.Attempts(uint(w.cfg.Retry)),
	)

	if err != nil {
		return "", errs.Wrapf(err, "retry do init multipart request failed, respStatus: %s", respStatus)
	}

	err = xml.Unmarshal(respBody.Bytes(), result)
	if err != nil {
		return "", errs.Wrap(err, "xml unmarshal fail")
	}

	return result.UploadID, nil
}

func (w *MultiPartWriter) uploadPart(partNo int, opts ...httpc.Option) *uploadPartResult {
	var (
		url         string
		checkStatus httpc.Option
		resp        *http.Response
		respStatus  string
		respBody    = bytes.NewBuffer(nil)
		etag        string
		err         error
	)
	if w.isBlob {
		blockID := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%08d", partNo)))
		etag = blockID
		url = fmt.Sprintf("%s?comp=block&blockid=%s", w.cfg.URL, blockID)
		checkStatus = httpc.CheckStatusCode(http.StatusCreated)
	} else {
		url = fmt.Sprintf("%s?partNumber=%d&uploadId=%s", w.cfg.URL, partNo, w.uploadID)
		checkStatus = httpc.CheckStatusCode(http.StatusOK)
	}

	resp, err = retry.DoWithData(
		func() (*http.Response, error) {
			respBody.Reset()
			return w.upload(url, append(opts, httpc.ToStatus(&respStatus), httpc.ToBytesBuffer(respBody), checkStatus)...)
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

	r := &uploadPartResult{
		partNo: partNo,
	}
	if err != nil {
		r.err = errs.Wrapf(err, "upload failed with max retry, respStatus: %s, respBody: %s", respStatus, respBody.String())
		return r
	}

	if !w.isBlob {
		etag = resp.Header.Get("Etag")
		if etag == "" {
			etag = resp.Header.Get("ETag")
		}
		if etag == "" {
			r.err = errs.Errorf("etag not exists in resp header, respStatus: %s, respBody: %s", respStatus, respBody.String())
			return r
		}

		r.part = &Part{
			PartNumber: partNo,
			ETag:       etag,
		}
	} else {
		r.block = etag
	}

	return r
}

func (w *MultiPartWriter) upload(url string, opts ...httpc.Option) (*http.Response, error) {
	allOpts := make([]httpc.Option, 0, len(opts)+1)
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, httpc.ReqOptionFunc(w.addAuth))

	return httpc.Put(w.ctx, 0, url, allOpts...)
}
