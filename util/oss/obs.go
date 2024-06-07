package oss

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var obsURLRegex = regexp.MustCompile(`([a-z0-9\-\.]{3,63})\.obs\..+\.com(/.*)`)

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

	stringToSign := req.Method + "\n"
	stringToSign += "\n"
	stringToSign += "\n"
	stringToSign += date + "\n"
	stringToSign += "/" + bucket + obj

	mac := hmac.New(sha1.New, []byte(sk))
	_, err := mac.Write([]byte(stringToSign))
	if err != nil {
		return err
	}

	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set(HeaderAuthorization, fmt.Sprintf("%s %s:%s", "OBS", ak, sign))
	req.Header.Set(HeaderDate, date)
	return nil
}
