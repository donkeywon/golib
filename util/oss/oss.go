package oss

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpu"
)

var commonTimeout = 10 * time.Second

func Which(url string) Type {
	if IsAzblob(url) {
		return TypeBlob
	}
	if IsObs(url) {
		return TypeOBS
	}
	if IsAmzS3(url) {
		return TypeAmazonS3
	}
	if IsAliOSS(url) {
		return TypeAliyunOSS
	}
	return whichByHead(url)
}

func whichByHead(url string) Type {
	resp, err := httpc.Head(context.Background(), time.Second*5, url)
	if err != nil {
		return TypeUnknown
	}
	serverHeader := resp.Header.Get(httpu.HeaderServer)
	if serverHeader == "" {
		return TypeUnknown
	}
	switch serverHeader {
	case string(TypeOBS):
		return TypeOBS
	case string(TypeAliyunOSS):
		return TypeAliyunOSS
	case string(TypeAmazonS3):
		return TypeAmazonS3
	case string(TypeMinIO):
		return TypeMinIO
	default:
		if strings.Contains(serverHeader, "Blob") {
			return TypeBlob
		}
		return TypeUnknown
	}
}

func NeedContentLength(url string) bool {
	switch Which(url) {
	case TypeOBS, TypeAliyunOSS:
		return false
	case TypeBlob, TypeAmazonS3, TypeMinIO, TypeUnknown:
		return true
	default:
		return true
	}
}

func IsSupportAppend(url string) bool {
	switch Which(url) {
	case TypeBlob, TypeOBS, TypeAliyunOSS:
		return true
	case TypeAmazonS3, TypeMinIO, TypeUnknown:
		return false
	default:
		return false
	}
}

func GetNextPositionFromResponse(resp *http.Response) (int, bool, error) {
	if resp == nil {
		return 0, false, nil
	}

	nextPositionHeader := resp.Header.Get(HeaderOSSAppendNextPositionHeader)
	if nextPositionHeader == "" {
		nextPositionHeader = resp.Header.Get(HeaderOBSAppendNextPositionHeader)
	}
	if nextPositionHeader == "" {
		nextPositionHeader = resp.Header.Get(HeaderAliOSSAppendNextPositionHeader)
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
	if IsAliOSS(req.URL.String()) {
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
		respStatus  string
		respBody    = bytes.NewBuffer(nil)
	)
	if IsAzblob(url) {
		checkStatus = []int{http.StatusAccepted, http.StatusNotFound}
	} else {
		checkStatus = []int{http.StatusNoContent}
	}

	_, err := httpc.Delete(ctx, timeout, url,
		httpc.ReqOptionFunc(func(req *http.Request) error {
			return Sign(req, ak, sk, region)
		}),
		httpc.CheckStatusCode(checkStatus...),
		httpc.ToStatus(&respStatus),
		httpc.ToBytesBuffer(respBody),
	)

	if err != nil {
		return errs.Errorf("do http delete request fail, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}

	return nil
}

func Head(ctx context.Context, timeout time.Duration, url string, ak string, sk string, region string) (*http.Response, error) {
	var (
		respStatus string
		respBody   = bytes.NewBuffer(nil)
	)

	resp, err := httpc.Head(ctx, timeout, url,
		httpc.ReqOptionFunc(func(req *http.Request) error {
			return Sign(req, ak, sk, region)
		}),
		httpc.CheckStatusCode(http.StatusOK),
		httpc.ToStatus(&respStatus),
		httpc.ToBytesBuffer(respBody),
	)

	if err != nil {
		return nil, errs.Errorf("do http head request fail, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}

	return resp, nil
}
