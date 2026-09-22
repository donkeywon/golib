package oss

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"codeberg.org/donkeywon/golib/errs"
	"codeberg.org/donkeywon/golib/util/httpc"
	"codeberg.org/donkeywon/golib/util/httpu"
	"codeberg.org/donkeywon/golib/util/ossu"
	"github.com/avast/retry-go/v4"
)

type MultiPartWriter struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    Cfg

	initialized bool
	isBlob      bool

	uploadID     string
	uploadFailed bool

	curPartNo int
	parts     []Part
	blockList []string

	parallelUploadPartReqCh chan *uploadPartReq
	bufCh                   chan []byte
	parallelResult          []uploadPartResult
	parallelErrs            loadOnceError
	parallelWg              sync.WaitGroup
	closeOnce               func() error
	closeCh                 chan struct{}
	mu                      sync.Mutex
}

func NewMultiPartWriter(ctx context.Context, cfg Cfg) *MultiPartWriter {
	cfg.setDefaults()
	w := &MultiPartWriter{
		ctx:     ctx,
		cfg:     cfg,
		isBlob:  ossu.IsAzblob(cfg.URL),
		closeCh: make(chan struct{}),
	}

	w.closeOnce = sync.OnceValue(func() error {
		var alreadyCancelled bool
		select {
		case <-w.ctx.Done():
			alreadyCancelled = true
		default:
		}

		close(w.closeCh)
		w.parallelWg.Wait()

		w.cancel()

		var err error
		if w.parallelErrs.Has() || w.uploadFailed || alreadyCancelled {
			err = w.parallelErrs.LoadOnce()
		} else {
			err = w.complete()
		}
		return err
	})
	return w
}

type uploadPartReq struct {
	b      []byte
	partNo int
}

var errWriterClosed = errors.New("multipart writer closed")

type uploadPartResult struct {
	err   error
	part  Part
	block string

	partNo int
}

func (w *MultiPartWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	case <-w.closeCh:
		return 0, errWriterClosed
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	err := w.init()
	if err != nil {
		return 0, err
	}

	if w.cfg.Parallel > 1 {
		err = w.parallelErrs.Load()
		if err != nil {
			return 0, err
		}

		var b []byte
		select {
		case b = <-w.bufCh:
			if cap(b) < len(p) {
				b = make([]byte, len(p))
			}
		default:
			b = make([]byte, len(p))
		}

		b = b[:cap(b)]
		nc := copy(b, p)
		req := &uploadPartReq{partNo: w.curPartNo, b: b[:nc]}
		select {
		case w.parallelUploadPartReqCh <- req:
			w.curPartNo++
			return len(p), nil
		case <-w.closeCh:
			return 0, errWriterClosed
		}
	}

	r := w.retryUploadPart(w.curPartNo, p)
	w.curPartNo++
	if r.err != nil {
		w.uploadFailed = true
		return 0, errs.Wrap(r.err, "upload part failed")
	}
	w.handleResult(r)

	return len(p), nil
}

func (w *MultiPartWriter) handleParallelResult(r uploadPartResult, workerID int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if r.err != nil {
		w.parallelErrs.Store(errs.Wrapf(r.err, "upload worker %d failed", workerID))
		return
	}

	w.parallelResult = append(w.parallelResult, r)
}

func (w *MultiPartWriter) handleResult(r uploadPartResult) {
	if w.isBlob {
		w.blockList = append(w.blockList, r.block)
	} else {
		w.parts = append(w.parts, r.part)
	}
}

func (w *MultiPartWriter) Close() error {
	if !w.initialized {
		return nil
	}

	return w.closeOnce()
}

func (w *MultiPartWriter) init() error {
	if w.initialized {
		return nil
	}

	var err error
	if !w.isBlob {
		w.uploadID, err = w.initMultiPart()
		if err != nil {
			return errs.Wrap(err, "init multi part failed")
		}
	}

	w.ctx, w.cancel = context.WithCancel(w.ctx)
	w.initialized = true
	w.curPartNo = 1
	if w.cfg.Parallel > 1 {
		w.bufCh = make(chan []byte, w.cfg.Parallel+1)
		w.parallelWg.Add(w.cfg.Parallel)
		w.parallelUploadPartReqCh = make(chan *uploadPartReq)
		for i := range w.cfg.Parallel {
			go w.uploadWorker(i)
		}
	}

	return nil
}

func (w *MultiPartWriter) uploadWorker(workerID int) {
	defer w.parallelWg.Done()
	for {
		select {
		case <-w.closeCh:
			return
		case req := <-w.parallelUploadPartReqCh:
			select {
			case <-w.ctx.Done():
				continue
			default:
			}
			if w.parallelErrs.Has() {
				w.recycleBuf(req.b)
				continue
			}

			r := w.retryUploadPart(req.partNo, req.b)
			w.recycleBuf(req.b)
			w.handleParallelResult(r, workerID)
		}
	}
}

func (w *MultiPartWriter) recycleBuf(b []byte) {
	select {
	case w.bufCh <- b:
	default:
	}
}

func (w *MultiPartWriter) complete() error {
	if w.cfg.Parallel > 1 {
		slices.SortFunc(w.parallelResult, func(a, b uploadPartResult) int {
			return a.partNo - b.partNo
		})
		if w.isBlob {
			w.blockList = make([]string, len(w.parallelResult))
			for i := range w.parallelResult {
				w.blockList[i] = w.parallelResult[i].block
			}
		} else {
			w.parts = make([]Part, len(w.parallelResult))
			for i := range w.parallelResult {
				w.parts[i] = w.parallelResult[i].part
			}
		}
	}

	if len(w.blockList) == 0 && len(w.parts) == 0 {
		return nil
	}

	var (
		url             string
		err             error
		body            any
		checkStatusCode int
		respStatusCode  int
		respBody        = bytes.NewBuffer(nil)
		contentType     string
		method          string
	)

	if w.isBlob {
		url = w.cfg.URL + "?comp=blocklist"
		checkStatusCode = http.StatusCreated
		body = &BlockList{Latest: w.blockList}
		contentType = httpu.MIMEPlainUTF8
		method = http.MethodPut
	} else {
		url = w.cfg.URL + "?uploadId=" + w.uploadID
		checkStatusCode = http.StatusOK
		body = &CompleteMultipartUpload{Parts: w.parts}
		contentType = httpu.MIMEXML
		method = http.MethodPost
	}

	err = retry.Do(
		func() error {
			respBody.Reset()
			return httpc.DoTimeout(context.Background(), w.cfg.Timeout, method, url,
				httpc.WithBodyMarshal(body, contentType, xml.Marshal),
				httpc.ReqOptionFunc(w.addAuth),
				httpc.CheckStatusCode(respBody, &respStatusCode, checkStatusCode),
			)
		},
		retry.LastErrorOnly(true),
		retry.Attempts(uint(w.cfg.Retry)),
	)

	if err != nil {
		return errs.Wrapf(err, "retry do complete multipart request fail, resp status code: %d, resp body: %s", respStatusCode, respBody.String())
	}
	return nil
}

func (w *MultiPartWriter) addAuth(req *http.Request) error {
	return ossu.Sign(req, w.cfg.Ak, w.cfg.Sk, w.cfg.Region)
}

func (w *MultiPartWriter) initMultiPart() (string, error) {
	var (
		respStatusCode int
		respBody       = bytes.NewBuffer(nil)
		err            error
	)
	result := &InitiateMultipartUploadResult{}
	err = retry.Do(
		func() error {
			respBody.Reset()
			return httpc.PostTimeout(w.ctx, w.cfg.Timeout, w.cfg.URL+"?uploads",
				httpc.ReqOptionFunc(w.addAuth),
				httpc.CheckStatusCode(respBody, &respStatusCode, http.StatusOK),
				httpc.ToWriter(respBody, nil),
			)
		},
		retry.LastErrorOnly(true),
		retry.Attempts(uint(w.cfg.Retry)),
		retry.Context(w.ctx),
	)

	if err != nil {
		return "", errs.Wrapf(err, "retry do init multipart request failed, resp status code: %d, resp body: %s", respStatusCode, respBody.String())
	}

	err = xml.Unmarshal(respBody.Bytes(), result)
	if err != nil {
		return "", errs.Wrap(err, "xml unmarshal fail")
	}

	return result.UploadID, nil
}

func (w *MultiPartWriter) retryUploadPart(partNo int, b []byte) uploadPartResult {
	r := uploadPartResult{partNo: partNo}
	err := retry.Do(
		func() error {
			r = w.uploadPart(partNo, httpc.WithBody(b))
			return r.err
		},
		retry.Attempts(uint(w.cfg.Retry)),
		retry.LastErrorOnly(true),
		retry.Context(w.ctx),
	)
	if err != nil && r.err == nil {
		r.err = err
	}
	return r
}

func (w *MultiPartWriter) uploadPart(partNo int, opts ...httpc.Option) uploadPartResult {
	var (
		url            string
		checkStatus    httpc.Option
		respStatusCode int
		respBody       = bytes.NewBuffer(nil)
		etag           string
		err            error
	)
	if w.isBlob {
		blockID := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%08d", partNo))
		etag = blockID
		url = fmt.Sprintf("%s?comp=block&blockid=%s", w.cfg.URL, blockID)
		checkStatus = httpc.CheckStatusCode(respBody, &respStatusCode, http.StatusCreated)
	} else {
		url = fmt.Sprintf("%s?partNumber=%d&uploadId=%s", w.cfg.URL, partNo, w.uploadID)
		checkStatus = httpc.CheckStatusCode(respBody, &respStatusCode, http.StatusOK)
	}

	err = w.upload(url, append(opts, checkStatus, httpc.RespOptionFunc(func(resp *http.Response) error {
		if !w.isBlob {
			etag = resp.Header.Get("Etag")
			if etag == "" {
				etag = resp.Header.Get("ETag")
			}
		}
		return nil
	}))...)

	r := uploadPartResult{
		partNo: partNo,
	}
	if err != nil {
		r.err = errs.Wrapf(err, "upload failed with max retry, resp status code: %d, resp body: %s", respStatusCode, respBody.String())
		return r
	}

	if !w.isBlob {
		if etag == "" {
			r.err = errs.Errorf("etag not exists in resp header, resp status code: %d, resp body: %s", respStatusCode, respBody.String())
			return r
		}

		r.part = Part{
			PartNumber: partNo,
			ETag:       etag,
		}
	} else {
		r.block = etag
	}

	return r
}

func (w *MultiPartWriter) upload(url string, opts ...httpc.Option) error {
	allOpts := make([]httpc.Option, 0, len(opts)+1)
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, httpc.ReqOptionFunc(w.addAuth))

	return httpc.PutTimeout(w.ctx, w.cfg.Timeout, url, allOpts...)
}

type loadOnceError struct {
	mu     sync.Mutex
	err    []error
	loaded bool
}

func (e *loadOnceError) Has() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.err) > 0
}

func (e *loadOnceError) LoadOnce() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.loaded {
		return nil
	}
	if len(e.err) == 0 {
		return nil
	}
	e.loaded = true
	return errors.Join(e.err...)
}

func (e *loadOnceError) Load() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.err) == 0 {
		return nil
	}
	e.loaded = true
	return errors.Join(e.err...)
}

func (e *loadOnceError) Store(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.err = append(e.err, err)
}

type InitiateMultipartUploadResult struct {
	UploadID string `xml:"UploadId"`
}

type CompleteMultipartUpload struct {
	Parts []Part `xml:"Part"`
}

type Part struct {
	ETag       string `xml:"ETag"`
	PartNumber int    `xml:"PartNumber"`
}

type BlockList struct {
	Latest []string `xml:"Latest"`
}
