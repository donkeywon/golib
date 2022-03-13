package oss

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"github.com/donkeywon/golib/util/httpu"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/donkeywon/golib/errs"
)

var obsURLRegex = regexp.MustCompile(`([a-z0-9\-\.]{3,63})\.obs\..+\.com(\/.*)`)

const obsURLMatchCount = 3

func IsObs(url string) bool {
	ok, _, _ := ParseObsURL(url)
	return ok
}

// ok, bucket, object.
func ParseObsURL(url string) (bool, string, string) {
	matchRes := obsURLRegex.FindStringSubmatch(url)
	if len(matchRes) != obsURLMatchCount {
		return false, "", ""
	}
	return true, matchRes[1], matchRes[2]
}

func ObsSign(req *http.Request, ak string, sk string, bucket string, obj string) error {
	date := time.Now().UTC().Format(time.RFC1123)
	date = date[:strings.LastIndex(date, "UTC")] + "GMT"
	contentType := req.Header.Get(httpu.HeaderContentType)

	stringToSign := req.Method + "\n"
	stringToSign += "\n"
	if contentType == "" {
		stringToSign += "\n"
	} else {
		stringToSign += contentType + "\n"
	}
	stringToSign += date + "\n"
	stringToSign += "/" + bucket + obj

	mac := hmac.New(sha1.New, []byte(sk))
	_, err := mac.Write([]byte(stringToSign))
	if err != nil {
		return errs.Wrap(err, "obs hmac write failed")
	}

	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set(httpu.HeaderAuthorization, fmt.Sprintf("%s %s:%s", "OBS", ak, sign))
	req.Header.Set(httpu.HeaderDate, date)
	return nil
}
