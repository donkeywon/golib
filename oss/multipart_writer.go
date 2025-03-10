package oss

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/donkeywon/golib/util/httpu"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
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
	*Cfg

	closed chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	uploadID  string
	curPartNo int

	parts     []*Part
	blockList []string

	uploadErr error

	initialized bool
}

func NewMultiPartWriter() *MultiPartWriter {
	w := &MultiPartWriter{
		Cfg:    NewCfg(),
		closed: make(chan struct{}),
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	return w
}

func (w *MultiPartWriter) Write(p []byte) (int, error) {
	select {
	case <-w.closed:
		return 0, ErrAlreadyClosed
	default:
	}

	if len(p) == 0 {
		return 0, nil
	}

	if w.Retry <= 0 {
		w.Retry = 3
	}

	var err error

	if !oss.IsAzblob(w.URL) && !w.initialized {
		w.uploadID, err = w.initMultiPart()
		if err != nil {
			return 0, errs.Wrap(err, "init multi part failed")
		}
		w.initialized = true
		w.curPartNo = 1
	}

	err = w.uploadPart(p)
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
	w.once.Do(func() {
		close(w.closed)
		w.cancel()

		if w.uploadErr == nil {
			err = w.Complete()
		} else {
			err = w.Abort()
		}
	})
	return err
}

func (w *MultiPartWriter) Abort() error {
	if oss.IsAzblob(w.URL) {
		return nil
	}

	var respBody = bytes.NewBuffer(nil)
	resp, err := retry.DoWithData(func() (*http.Response, error) {
		return httpc.Delete(nil, time.Second*time.Duration(w.Timeout), w.URL+"?uploadId="+w.uploadID,
			httpc.ReqOptionFunc(w.addAuth),
			httpc.CheckStatusCode(http.StatusNoContent),
			httpc.ToBytesBuffer(nil, respBody))
	}, retry.Attempts(uint(w.Retry)))

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
	)

	if oss.IsAzblob(w.URL) {
		url = w.URL + "?comp=blocklist"
		checkStatus = http.StatusCreated
		body = &BlockList{Latest: w.blockList}
		contentType = httpu.MIMEPlainUTF8
	} else {
		url = w.URL + "?uploadId=" + w.uploadID
		checkStatus = http.StatusOK
		body = &CompleteMultipartUpload{Parts: w.parts}
		contentType = httpu.MIMEXML
	}

	resp, err = retry.DoWithData(func() (*http.Response, error) {
		return httpc.Put(nil, time.Second*time.Duration(w.Timeout), url,
			httpc.ReqOptionFunc(w.addAuth),
			httpc.WithBodyMarshal(body, contentType, xml.Marshal),
			httpc.CheckStatusCode(checkStatus),
			httpc.ToBytesBuffer(nil, respBody),
		)
	}, retry.Attempts(uint(w.Retry)))

	if err != nil {
		return errs.Wrapf(err, "retry do complete multipart request fail, resp: %+v, body: %s", resp, respBody.String())
	}
	return nil
}

func (w *MultiPartWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.Ak, w.Sk, w.Region)
}

func (w *MultiPartWriter) initMultiPart() (string, error) {
	result := &InitiateMultipartUploadResult{}
	resp, err := retry.DoWithData(
		func() (*http.Response, error) {
			return httpc.Post(w.ctx, time.Second*time.Duration(w.Timeout), w.URL+"?uploads",
				httpc.ReqOptionFunc(w.addAuth),
				httpc.CheckStatusCode(http.StatusOK),
				httpc.ToAnyUnmarshal(result, xml.Unmarshal),
			)
		},
		retry.Attempts(uint(w.Retry)),
	)

	if err != nil {
		return "", errs.Wrapf(err, "retry do init multipart request fail, resp: %+v", resp)
	}

	return result.UploadID, nil
}

func (w *MultiPartWriter) uploadPart(body []byte) error {
	etag, err := retry.DoWithData(
		func() (string, error) {
			return w.upload(w.curPartNo, w.uploadID, body)
		},
		retry.Attempts(uint(w.Retry)),
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
		return errs.Wrap(err, "upload fail with max retry")
	}

	if oss.IsAzblob(w.URL) {
		w.blockList = append(w.blockList, etag)
		return nil
	}

	w.parts = append(w.parts, &Part{
		PartNumber: w.curPartNo,
		ETag:       etag,
	})
	w.curPartNo++
	return nil
}

func (w *MultiPartWriter) upload(partNo int, uploadID string, body []byte) (string, error) {
	var (
		url         string
		etag        string
		checkStatus int
		respBody    = bytes.NewBuffer(nil)
	)
	if oss.IsAzblob(w.URL) {
		blockID := base64.StdEncoding.EncodeToString([]byte(uuid.NewString()))
		etag = blockID
		url = fmt.Sprintf("%s?comp=block&blockid=%s", w.URL, blockID)
		checkStatus = http.StatusCreated
	} else {
		url = fmt.Sprintf("%s?partNumber=%d&uploadId=%s", w.URL, partNo, uploadID)
		checkStatus = http.StatusOK
	}

	resp, err := httpc.Put(w.ctx, time.Second*time.Duration(w.Timeout), url,
		httpc.ReqOptionFunc(w.addAuth),
		httpc.WithBody(body),
		httpc.CheckStatusCode(checkStatus),
		httpc.ToBytesBuffer(nil, respBody),
	)

	if err != nil {
		return "", errs.Wrapf(err, "do upload request fail, resp: %+v, respBody: %s", resp, respBody.String())
	}

	if oss.IsAzblob(w.URL) {
		return etag, nil
	}

	etag = resp.Header.Get("Etag")
	if etag == "" {
		etag = resp.Header.Get("ETag")
	}
	if etag == "" {
		return "", errs.Errorf("etag not exists in resp header, resp: %+v, respBody: %s", resp, respBody.String())
	}

	return etag, nil
}
