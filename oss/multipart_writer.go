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
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpio"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/donkeywon/golib/util/oss"
	"github.com/google/uuid"
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
	cfg               *Cfg
	w                 *multiPartWriter
	bufw              *bufio.Writer
	needContentLength bool
}

func NewMultiPartWriter(ctx context.Context, cfg *Cfg) *MultiPartWriter {
	cfg.setDefaults()
	w := &multiPartWriter{
		cfg: cfg,
	}
	w.timeout = time.Second * time.Duration(w.cfg.Timeout)
	w.ctx, w.cancel = context.WithCancel(ctx)

	return &MultiPartWriter{
		cfg:               cfg,
		w:                 w,
		needContentLength: oss.NeedContentLength(cfg.URL),
	}
}

func (w *MultiPartWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

func (w *MultiPartWriter) Close() error {
	if w.bufw != nil {
		return errors.Join(w.bufw.Flush(), w.w.Close())
	}
	return w.w.Close()
}

func (w *MultiPartWriter) ReadFrom(r io.Reader) (int64, error) {
	select {
	case <-w.w.ctx.Done():
		return 0, httpio.ErrAlreadyClosed
	default:
	}

	err := w.w.init()
	if err != nil {
		return 0, err
	}

	if w.needContentLength {
		w.bufw = bufio.NewWriterSize(w.w, int(w.cfg.PartSize))
		return io.Copy(w.bufw, r)
	}

	rr := &readerWrapper{Reader: r}
	for {
		lr := io.LimitReader(rr, w.cfg.PartSize)
		err = w.w.uploadPart(httpc.WithBodyReader(lr))
		if err != nil {
			break
		}
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
	r.eof = errors.Is(err, io.EOF)
	return nr, err
}

type multiPartWriter struct {
	cfg *Cfg

	timeout   time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	uploadID  string
	curPartNo int

	parts     []*Part
	blockList []string

	uploadErr error

	initialized       bool
	needContentLength bool
}

func (w *multiPartWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, httpio.ErrAlreadyClosed
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	err := w.init()
	if err != nil {
		return 0, err
	}

	err = w.uploadPart(httpc.WithBody(p))
	if err != nil {
		if w.uploadErr == nil {
			w.uploadErr = err
		}
		return 0, errs.Wrap(err, "upload part failed")
	}

	return len(p), nil
}

func (w *multiPartWriter) Close() error {
	var err error
	w.closeOnce.Do(func() {
		w.cancel()

		if w.uploadErr == nil {
			err = w.Complete()
		} else {
			err = w.Abort()
		}
	})
	return err
}

func (w *multiPartWriter) init() error {
	if w.initialized {
		return nil
	}

	w.needContentLength = oss.NeedContentLength(w.cfg.URL)

	if oss.IsAzblob(w.cfg.URL) {
		w.initialized = true
		return nil
	}

	var err error
	w.uploadID, err = w.initMultiPart()
	if err != nil {
		return errs.Wrap(err, "init multi part failed")
	}
	w.initialized = true
	w.curPartNo = 1
	return nil
}

func (w *multiPartWriter) Abort() error {
	if oss.IsAzblob(w.cfg.URL) {
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
	)

	if err != nil {
		return errs.Wrapf(err, "abort multipart fail, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}
	return nil
}

func (w *multiPartWriter) Complete() error {
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

	if oss.IsAzblob(w.cfg.URL) {
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

func (w *multiPartWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.cfg.Ak, w.cfg.Sk, w.cfg.Region)
}

func (w *multiPartWriter) initMultiPart() (string, error) {
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
		retry.Attempts(uint(w.cfg.Retry)),
	)

	if err != nil {
		return "", errs.Wrapf(err, "retry do init multipart request fail, respStatus: %s", respStatus)
	}

	err = xml.Unmarshal(respBody.Bytes(), result)
	if err != nil {
		return "", errs.Wrap(err, "xml unmarshal fail")
	}

	return result.UploadID, nil
}

func (w *multiPartWriter) uploadPart(opts ...httpc.Option) error {
	var (
		url         string
		checkStatus httpc.Option
		resp        *http.Response
		respStatus  string
		respBody    = bytes.NewBuffer(nil)
		etag        string
		err         error
	)
	if oss.IsAzblob(w.cfg.URL) {
		blockID := base64.StdEncoding.EncodeToString([]byte(uuid.NewString()))
		etag = blockID
		url = fmt.Sprintf("%s?comp=block&blockid=%s", w.cfg.URL, blockID)
		checkStatus = httpc.CheckStatusCode(http.StatusCreated)
	} else {
		url = fmt.Sprintf("%s?partNumber=%d&uploadId=%s", w.cfg.URL, w.curPartNo, w.uploadID)
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
	if err != nil {
		return errs.Wrapf(err, "upload failed with max retry, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}

	if !oss.IsAzblob(w.cfg.URL) {
		etag = resp.Header.Get("Etag")
		if etag == "" {
			etag = resp.Header.Get("ETag")
		}
		if etag == "" {
			return errs.Errorf("etag not exists in resp header, respStatus: %s, respBody: %s", respStatus, respBody.String())
		}

		w.parts = append(w.parts, &Part{
			PartNumber: w.curPartNo,
			ETag:       etag,
		})
		w.curPartNo++
	} else {
		w.blockList = append(w.blockList, etag)
	}

	return nil
}

func (w *multiPartWriter) upload(url string, opts ...httpc.Option) (*http.Response, error) {
	allOpts := make([]httpc.Option, 0, len(opts)+1)
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts,
		httpc.ReqOptionFunc(w.addAuth),
	)

	return httpc.Put(w.ctx, w.timeout, url, allOpts...)
}
