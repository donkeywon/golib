package oss

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
)

var azblobURLSuffix = "core.windows.net"

func CreateAppendBlob(ctx context.Context, url string, ak string, sk string) error {
	var respBody = bytes.NewBuffer(nil)
	resp, err := httpc.Put(ctx, commonTimeout, url,
		httpc.WithHeaders(HeaderXmsBlobType, "AppendBlob"),
		azblobSignOption(ak, sk),
		httpc.CheckStatusCode(http.StatusCreated),
		httpc.ToBytesBuffer(respBody))

	if err != nil {
		return errs.Wrapf(err, "do http request create append blob fail: %v, body: %s", resp, respBody.String())
	}

	return nil
}

func azblobSignOption(ak, sk string) httpc.Option {
	return httpc.ReqOptionFunc(func(r *http.Request) error {
		return AzblobSign(r, ak, sk)
	})
}

func AzblobSign(req *http.Request, account string, key string) error {
	req.Header.Set(HeaderXmsDate, time.Now().UTC().Format(http.TimeFormat))
	if req.Header.Get(HeaderContentLength) == "" {
		req.Header.Set(HeaderContentLength, strconv.Itoa(int(req.ContentLength)))
	}
	req.Header.Set(HeaderXmsVersion, "2023-11-03")

	stringToSign, err := azblobBuildStringToSign(req, account)
	if err != nil {
		return errs.Wrap(err, "azblob build string to sign failed")
	}

	keyBs, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return errs.Wrap(err, "account key is invalid base64 encoded string")
	}

	mac := hmac.New(sha256.New, keyBs)
	_, err = mac.Write([]byte(stringToSign))
	if err != nil {
		return errs.Wrap(err, "azblob hmac write failed")
	}

	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set(HeaderAuthorization, fmt.Sprintf("SharedKey %s:%s", account, sign))
	return nil
}

func IsAzblob(url string) bool {
	return strings.Contains(url, azblobURLSuffix)
}

// below copied from github.com/Azure/azure-sdk-for-go

func azblobBuildStringToSign(req *http.Request, accountName string) (string, error) {
	// https://docs.microsoft.com/en-us/rest/api/storageservices/authentication-for-the-azure-storage-services
	headers := req.Header
	contentLength := getHeader(HeaderContentLength, headers)
	if contentLength == "0" {
		contentLength = ""
	}

	canonicalizedResource, err := azblobBuildCanonicalizedResource(req.URL, accountName)
	if err != nil {
		return "", err
	}

	stringToSign := strings.Join([]string{
		req.Method,
		getHeader(HeaderContentEncoding, headers),
		getHeader(HeaderContentLanguage, headers),
		contentLength,
		getHeader(HeaderContentMD5, headers),
		getHeader(HeaderContentType, headers),
		"", // Empty date because x-ms-date is expected (as per web page above)
		getHeader(HeaderIfModifiedSince, headers),
		getHeader(HeaderIfMatch, headers),
		getHeader(HeaderIfNoneMatch, headers),
		getHeader(HeaderIfUnmodifiedSince, headers),
		getHeader(HeaderRange, headers),
		azblobBuildCanonicalizedHeader(headers),
		canonicalizedResource,
	}, "\n")
	return stringToSign, nil
}

func getHeader(key string, headers map[string][]string) string {
	if headers == nil {
		return ""
	}
	if v, ok := headers[key]; ok {
		if len(v) > 0 {
			return v[0]
		}
	}

	return ""
}

func azblobBuildCanonicalizedHeader(headers http.Header) string {
	cm := map[string][]string{}
	for k, v := range headers {
		headerName := strings.TrimSpace(strings.ToLower(k))
		if strings.HasPrefix(headerName, "x-ms-") {
			cm[headerName] = v // NOTE: the value must not have any whitespace around it.
		}
	}
	if len(cm) == 0 {
		return ""
	}

	keys := make([]string, 0, len(cm))
	for key := range cm {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ch := bytes.NewBufferString("")
	for i, key := range keys {
		if i > 0 {
			ch.WriteRune('\n')
		}
		ch.WriteString(key)
		ch.WriteRune(':')
		ch.WriteString(strings.Join(cm[key], ","))
	}
	return ch.String()
}

func azblobBuildCanonicalizedResource(u *url.URL, accountName string) (string, error) {
	// https://docs.microsoft.com/en-us/rest/api/storageservices/authentication-for-the-azure-storage-services
	cr := bytes.NewBufferString("/")
	cr.WriteString(accountName)

	if len(u.Path) > 0 {
		// Any portion of the CanonicalizedResource string that is derived from
		// the resource's URI should be encoded exactly as it is in the URI.
		// -- https://msdn.microsoft.com/en-gb/library/azure/dd179428.aspx
		cr.WriteString(u.EscapedPath())
	} else {
		// a slash is required to indicate the root path
		cr.WriteString("/")
	}

	// params is a map[string][]string; param name is key; params values is []string
	params, err := url.ParseQuery(u.RawQuery) // Returns URL decoded values
	if err != nil {
		return "", fmt.Errorf("failed to parse query params: %w", err)
	}

	if len(params) > 0 { // There is at least 1 query parameter
		var paramNames []string // We use this to sort the parameter key names
		for paramName := range params {
			paramNames = append(paramNames, paramName) // paramNames must be lowercase
		}
		sort.Strings(paramNames)

		for _, paramName := range paramNames {
			paramValues := params[paramName]
			sort.Strings(paramValues)

			// Join the sorted key values separated by ','
			// Then prepend "keyName:"; then add this string to the buffer
			cr.WriteString("\n" + strings.ToLower(paramName) + ":" + strings.Join(paramValues, ","))
		}
	}
	return cr.String(), nil
}
