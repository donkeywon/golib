package oss

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpu"
)

var commonTimeout = 10 * time.Second

var NeedContentLength = func(url string) bool {
	switch Which(url) {
	case TypeOBS, TypeAliyunOSS:
		return false
	case TypeBlob, TypeAmazonS3, TypeMinIO, TypeUnknown:
		return true
	default:
		return true
	}
}

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
		httpc.ToStatus(&respStatus),
		httpc.ToBytesBuffer(respBody),
		httpc.CheckStatusCode(checkStatus...),
	)

	if err != nil {
		return errs.Wrapf(err, "http delete failed, respStatus: %s, respBody: %s", respStatus, respBody.String())
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
		return nil, errs.Wrapf(err, "http head failed, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}

	return resp, nil
}

type ListBucketResult struct {
	Name        string    `xml:"Name"`
	Prefix      string    `xml:"Prefix"`
	KeyCount    int       `xml:"KeyCount"`
	MaxKeys     int       `xml:"MaxKeys"`
	IsTruncated bool      `xml:"IsTruncated"`
	Contents    []Content `xml:"Contents"`
}

type Content struct {
	ETag         string `xml:"ETag"`
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	Size         int64  `xml:"Size"`
}

type listBlobResult struct {
	Blobs         []azblob `xml:"Blobs>Blob"`
	Prefix        string   `xml:"Prefix"`
	ContainerName string   `xml:"ContainerName,attr"`
	MaxResults    int      `xml:"MaxResults"`
}

type azblob struct {
	Name       string         `xml:"Name"`
	Properties blobProperties `xml:"Properties"`
}

type blobProperties struct {
	ContentLength int64  `xml:"Content-Length"`
	LastModified  string `xml:"Last-Modified"`
	ETag          string `xml:"Etag"`
}

func List(ctx context.Context, timeout time.Duration, url string, ak string, sk string, region string) (ListBucketResult, error) {
	var (
		respStatus string
		respBody   = bytes.NewBuffer(nil)
		result     ListBucketResult
		azResult   listBlobResult
	)

	_, err := httpc.Get(ctx, timeout, url,
		httpc.ReqOptionFunc(func(r *http.Request) error {
			return Sign(r, ak, sk, region)
		}),
		httpc.CheckStatusCode(http.StatusOK),
		httpc.ToStatus(&respStatus),
		httpc.ToBytesBuffer(respBody),
	)

	if err != nil {
		return result, errs.Wrapf(err, "http get failed, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}

	if !IsAzblob(url) {
		err = xml.Unmarshal(respBody.Bytes(), &result)
		if err != nil {
			return result, errs.Wrapf(err, "parse resp to result failed, respStatus: %s, respBody: %s", respStatus, respBody.String())
		}

		return result, nil

	}

	err = xml.Unmarshal(respBody.Bytes(), &azResult)
	if err != nil {
		return result, errs.Wrapf(err, "parse resp to result failed, respStatus: %s, respBody: %s", respStatus, respBody.String())
	}
	result.Name = azResult.ContainerName
	result.Prefix = azResult.Prefix
	result.Contents = make([]Content, len(azResult.Blobs))
	result.KeyCount = len(azResult.Blobs)
	if azResult.MaxResults == 0 {
		result.MaxKeys = result.KeyCount
	} else {
		result.MaxKeys = azResult.MaxResults
	}

	for i := range azResult.Blobs {
		result.Contents[i] = Content{
			ETag:         azResult.Blobs[i].Properties.ETag,
			Key:          azResult.Blobs[i].Name,
			LastModified: azResult.Blobs[i].Properties.LastModified,
			Size:         azResult.Blobs[i].Properties.ContentLength,
		}
	}
	return result, nil
}
