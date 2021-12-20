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
	"time"

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
		w.uploadID, err = retry.DoWithData(
			func() (string, error) {
				return w.initMultiPart()
			},
			retry.Attempts(uint(w.Retry)),
		)
		if err != nil {
			return 0, errs.Wrap(err, "init multi part fail")
		}
		w.initialized = true
		w.curPartNo = 1
	}

	err = w.uploadPart(p)
	if err != nil {
		if w.uploadErr == nil {
			w.uploadErr = err
		}
		return 0, errs.Wrap(err, "upload part fail")
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

	url := w.URL + "?uploadId=" + w.uploadID

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(w.Timeout))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errs.Wrap(err, "new abort multipart request fail")
	}

	err = w.addAuth(req)
	if err != nil {
		return errs.Wrap(err, "sign oss req fail")
	}

	body, resp, err := w.do(req, http.StatusNoContent)
	if err != nil {
		return errs.Wrapf(err, "abort multipart fail, resp: %+v, body: %s", resp, string(body))
	}

	return nil
}

func (w *MultiPartWriter) Complete() error {
	var (
		url    string
		bs     []byte
		err    error
		method string
	)

	if oss.IsAzblob(w.URL) {
		url = w.URL + "?comp=blocklist"

		res := &BlockList{Latest: w.blockList}
		bs, err = xml.Marshal(res)
		if err != nil {
			return errs.Wrapf(err, "marshal BlockList fail: %+v", res)
		}
		method = http.MethodPut
	} else {
		url = w.URL + "?uploadId=" + w.uploadID

		res := &CompleteMultipartUpload{Parts: w.parts}
		bs, err = xml.Marshal(res)
		if err != nil {
			return errs.Wrapf(err, "marshal CompleteMultipartUpload fail: %+v", res)
		}
		method = http.MethodPost
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(w.Timeout))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bs))
	if err != nil {
		return errs.Wrap(err, "new complete multipart request fail")
	}

	err = w.addAuth(req)
	if err != nil {
		return errs.Wrap(err, "sign oss req fail")
	}

	body, resp, err := w.do(req)
	if err != nil {
		return errs.Wrapf(err, "do complete multipart request fail, resp: %+v, body: %s", resp, string(body))
	}

	return nil
}

func (w *MultiPartWriter) addAuth(req *http.Request) error {
	return oss.Sign(req, w.Ak, w.Sk, w.Region)
}

func (w *MultiPartWriter) do(req *http.Request, ignoreCode ...int) ([]byte, *http.Response, error) {
	body, resp, err := httpc.DoBody(req)
	if errors.Is(err, context.Canceled) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, errs.Wrap(err, "do write http request fail")
	}

	if oss.IsAzblob(w.URL) {
		if resp.StatusCode != http.StatusCreated && !slices.Contains(ignoreCode, resp.StatusCode) {
			return body, resp, errs.Errorf("http resp status code is not created: %d, body: %s", resp.StatusCode, string(body))
		}
	} else {
		if resp.StatusCode != http.StatusOK && !slices.Contains(ignoreCode, resp.StatusCode) {
			return body, resp, errs.Errorf("http resp status code is not ok: %d, body: %s", resp.StatusCode, string(body))
		}
	}

	return body, resp, nil
}

func (w *MultiPartWriter) initMultiPart() (string, error) {
	url := w.URL + "?uploads"

	ctx, cancel := context.WithTimeout(w.ctx, time.Second*time.Duration(w.Timeout))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", errs.Wrap(err, "new init multipart request fail")
	}

	err = w.addAuth(req)
	if err != nil {
		return "", errs.Wrap(err, "sign oss req fail")
	}

	body, resp, err := w.do(req)
	if err != nil {
		return "", errs.Wrapf(err, "do req fail, resp: %+v, resp body: %s", resp, string(body))
	}

	res := &InitiateMultipartUploadResult{}
	err = xml.Unmarshal(body, res)
	if err != nil {
		return "", errs.Wrapf(err, "unmarshal multipart response xml fail, resp: %+v, resp body: %s", resp, string(body))
	}

	return res.UploadID, nil
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
		url  string
		etag string
	)
	if oss.IsAzblob(w.URL) {
		blockID := base64.StdEncoding.EncodeToString([]byte(uuid.NewString()))
		etag = blockID
		url = fmt.Sprintf("%s?comp=block&blockid=%s", w.URL, blockID)
	} else {
		url = fmt.Sprintf("%s?partNumber=%d&uploadId=%s", w.URL, partNo, uploadID)
	}

	ctx, cancel := context.WithTimeout(w.ctx, time.Second*time.Duration(w.Timeout))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return "", errs.Wrap(err, "new upload request fail")
	}

	req.ContentLength = int64(len(body))

	err = w.addAuth(req)
	if err != nil {
		return "", errs.Wrap(err, "sign oss req fail")
	}

	_, resp, err := w.do(req)
	if err != nil {
		return "", errs.Wrap(err, "do upload request fail")
	}

	if oss.IsAzblob(w.URL) {
		return etag, nil
	}

	etag = resp.Header.Get("Etag")
	if etag == "" {
		etag = resp.Header.Get("ETag")
	}
	if etag == "" {
		return "", errs.Errorf("etag not exists in resp header: %+v", resp)
	}

	return etag, nil
}
