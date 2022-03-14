package oss

import (
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

	initialized bool
}

func NewMultiPartWriter(ctx context.Context, cfg *Cfg) *MultiPartWriter {
	w := &MultiPartWriter{
		cfg: cfg,
	}
	cfg.setDefaults()
	w.timeout = time.Second * time.Duration(w.cfg.Timeout)
	w.ctx, w.cancel = context.WithCancel(ctx)
	return w
}

func (w *MultiPartWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, httpio.ErrAlreadyClosed
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	var err error

	if !oss.IsAzblob(w.cfg.URL) && !w.initialized {
		w.uploadID, err = w.initMultiPart()
		if err != nil {
			return 0, errs.Wrap(err, "init multi part failed")
		}
		w.initialized = true
		w.curPartNo = 1
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

func (w *MultiPartWriter) Close() error {
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

func (w *MultiPartWriter) ReadFrom(r io.Reader) (int64, error) {
	var (
		err error
		rr  = &readerWrapper{Reader: r}
	)

	for {
		lr := io.LimitReader(rr, w.cfg.PartSize)
		err = retry.Do(
			func() error {
				return w.uploadPart(httpc.WithBodyReader(lr))
			},
			retry.Attempts(uint(w.cfg.Retry)),
		)
		if err != nil {
			break
		}
		if rr.eof {
			break
		}
	}

	return rr.nr, err
}

func (w *MultiPartWriter) Abort() error {
	if oss.IsAzblob(w.cfg.URL) {
		return nil
	}

	var respBody = bytes.NewBuffer(nil)
	resp, err := retry.DoWithData(func() (*http.Response, error) {
		return httpc.Delete(nil, w.timeout, w.cfg.URL+"?uploadId="+w.uploadID,
			httpc.ReqOptionFunc(w.addAuth),
			httpc.CheckStatusCode(http.StatusNoContent),
			httpc.ToBytesBuffer(nil, respBody))
	}, retry.Attempts(uint(w.cfg.Retry)))

	if err != nil {
		return errs.Wrapf(err, "abort multipart fail, resp: %+v, body: %s", resp, respBody.String())
	}
	return nil
}

func (w *MultiPartWriter) Complete() error {
	var (
		url         string
		err         error
		body        any
		checkStatus int
		resp        *http.Response
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

	resp, err = retry.DoWithData(func() (*http.Response, error) {
		return httpc.Do(nil, w.timeout, method, url,
			httpc.WithBodyMarshal(body, contentType, xml.Marshal),
			httpc.ReqOptionFunc(w.addAuth),
			httpc.CheckStatusCode(checkStatus),
			httpc.ToBytesBuffer(nil, respBody),
		)
	}, retry.Attempts(uint(w.cfg.Retry)))

	if err != nil {
		return errs.Wrapf(err, "retry do complete multipart request fail, resp: %+v, body: %s", resp, respBody.String())
	}
	return nil
}

func (w *MultiPartWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.cfg.Ak, w.cfg.Sk, w.cfg.Region)
}

func (w *MultiPartWriter) initMultiPart() (string, error) {
	result := &InitiateMultipartUploadResult{}
	resp, err := retry.DoWithData(
		func() (*http.Response, error) {
			return httpc.Post(w.ctx, w.timeout, w.cfg.URL+"?uploads",
				httpc.ReqOptionFunc(w.addAuth),
				httpc.CheckStatusCode(http.StatusOK),
				httpc.ToAnyUnmarshal(result, xml.Unmarshal),
			)
		},
		retry.Attempts(uint(w.cfg.Retry)),
	)

	if err != nil {
		return "", errs.Wrapf(err, "retry do init multipart request fail, resp: %+v", resp)
	}

	return result.UploadID, nil
}

func (w *MultiPartWriter) uploadPart(opts ...httpc.Option) error {
	var (
		url         string
		checkStatus httpc.Option
		respStatus  string
		respBody    = bytes.NewBuffer(nil)
		etag        string
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

	resp, err := retry.DoWithData(
		func() (*http.Response, error) {
			return w.upload(url, append(opts, checkStatus)...)
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
		if resp != nil {
			respStatus = resp.Status
		}
		return errs.Wrapf(err, "upload failed with max retry, status: %s, resp: %s", respStatus, respBody.String())
	}

	if !oss.IsAzblob(w.cfg.URL) {
		etag = resp.Header.Get("Etag")
		if etag == "" {
			etag = resp.Header.Get("ETag")
		}
		if etag == "" {
			return errs.Errorf("etag not exists in resp header, status: %s, headers: %+v, respBody: %s", respStatus, resp.Header, respBody.String())
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

func (w *MultiPartWriter) upload(url string, opts ...httpc.Option) (*http.Response, error) {
	allOpts := make([]httpc.Option, 0, len(opts)+1)
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts,
		httpc.ReqOptionFunc(w.addAuth),
	)

	return httpc.Put(w.ctx, w.timeout, url, allOpts...)
}
