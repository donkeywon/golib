package oss

import (
	"context"
	"net/http"
	"strconv"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
)

func Which(url string) Type {
	if IsAzblob(url) {
		return OssTypeBlob
	}
	if IsObs(url) {
		return OssTypeObs
	}
	if IsAmzS3(url) {
		return OssTypeAmz
	}
	if IsAliOss(url) {
		return OssTypeAliOss
	}
	return OssTypeUnknown
}

func IsSupportAppend(url string) bool {
	switch Which(url) {
	case OssTypeBlob, OssTypeObs, OssTypeAliOss:
		return true
	case OssTypeAmz, OssTypeUnknown:
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

func Delete(ctx context.Context, url string, ak string, sk string, region string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	err = Sign(req, ak, sk, region)
	if err != nil {
		return err
	}

	body, resp, err := httpc.DoBody(req)
	if err != nil {
		return err
	}

	if IsAzblob(url) {
		if resp == nil || (resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNotFound) {
			return errs.Errorf("http resp status code is not accepted: %d, body: %s", resp.StatusCode, string(body))
		}
	} else {
		if resp == nil || resp.StatusCode != http.StatusNoContent {
			return errs.Errorf("http resp status code is not ok: %d, body: %s", resp.StatusCode, string(body))
		}
	}

	return nil
}

func Head(ctx context.Context, url string, ak string, sk string, region string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}

	if ak != "" || sk != "" {
		err = Sign(req, ak, sk, region)
		if err != nil {
			return nil, err
		}
	}

	resp, err := httpc.Do(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errs.Errorf("http resp status code is not ok: %d, resp: %+v", resp.StatusCode, resp)
	}

	return resp, nil
}
