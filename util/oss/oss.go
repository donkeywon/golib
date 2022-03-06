package oss

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
)

var commonTimeout = 10 * time.Second

func Which(url string) Type {
	if IsAzblob(url) {
		return TypeBlob
	}
	if IsObs(url) {
		return TypeObs
	}
	if IsAmzS3(url) {
		return TypeAmz
	}
	if IsAliOss(url) {
		return TypeAliOSS
	}
	return TypeUnknown
}

func IsSupportAppend(url string) bool {
	switch Which(url) {
	case TypeBlob, TypeObs, TypeAliOSS:
		return true
	case TypeAmz, TypeUnknown:
		return false
	default:
		return false
	}
}

func GetNextPositionFromResponse(resp *http.Response) (int, bool, error) {
	if resp == nil {
		return 0, false, nil
	}

	nextPositionHeader := resp.Header.Get(HeaderOssAppendNextPositionHeader)
	if nextPositionHeader == "" {
		nextPositionHeader = resp.Header.Get(HeaderObsAppendNextPositionHeader)
	}
	if nextPositionHeader == "" {
		nextPositionHeader = resp.Header.Get(HeaderAliOssAppendNextPositionHeader)
	}
	if nextPositionHeader == "" {
		return 0, false, nil
	}
	pos, err := strconv.Atoi(nextPositionHeader)
	if err != nil {
		return 0, true, err
	}
	return pos, true, nil
}

func Sign(req *http.Request, ak string, sk string, region string) error {
	if isObs, bucket, object := ParseObsURL(req.URL.String()); isObs {
		return ObsSign(req, ak, sk, bucket, object)
	}
	if IsAzblob(req.URL.String()) {
		return AzblobSign(req, ak, sk)
	}
	if IsAliOss(req.URL.String()) {
		return AliSign(req, ak, sk, region)
	}
	if IsAmzS3(req.URL.String()) {
		return AmzSign(req, ak, sk, region)
	}
	return AmzSign(req, ak, sk, region)
}

func Delete(ctx context.Context, timeout time.Duration, url string, ak string, sk string, region string) error {
	var (
		checkStatus []int
		respBody    = bytes.NewBuffer(nil)
	)
	if IsAzblob(url) {
		checkStatus = []int{http.StatusAccepted, http.StatusNotFound}
	} else {
		checkStatus = []int{http.StatusNoContent}
	}

	resp, err := httpc.Delete(ctx, timeout, url,
		httpc.ReqOptionFunc(func(req *http.Request) error {
			return Sign(req, ak, sk, region)
		}),
		httpc.CheckStatusCode(checkStatus...),
		httpc.ToBytesBuffer(respBody),
	)

	if err != nil {
		return errs.Errorf("do http delete request fail, resp: %+v, respBody: %s", resp, respBody.String())
	}

	return nil
}

func Head(ctx context.Context, timeout time.Duration, url string, ak string, sk string, region string) (*http.Response, error) {
	resp, err := httpc.Head(ctx, timeout, url,
		httpc.ReqOptionFunc(func(req *http.Request) error {
			return Sign(req, ak, sk, region)
		}),
		httpc.CheckStatusCode(http.StatusOK),
	)

	if err != nil {
		return nil, errs.Errorf("do http head request fail, resp: %+v", resp)
	}

	return resp, nil
}
