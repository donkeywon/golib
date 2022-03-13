package oss

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/donkeywon/golib/util/httpu"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	productOSS = "oss"

	aliOSSDomainSuffix = ".aliyuncs.com"
)

var aliOSSURLRegex = regexp.MustCompile(`([a-z0-9\-]{3,63})\.[^\.]+\.aliyuncs\.com(\/.*)`)

func IsAliOSS(url string) bool {
	return strings.Contains(url, aliOSSDomainSuffix)
}

func ParseAliOSSURL(url string) (bool, string, string) {
	matchRes := aliOSSURLRegex.FindStringSubmatch(url)
	if len(matchRes) != 3 {
		return false, "", ""
	}
	return true, matchRes[1], matchRes[2]
}

func AliSign(req *http.Request, ak, sk, region string) error {
	query := req.URL.Query()
	headers := req.Header

	now := time.Now()
	nowShortFormat := now.UTC().Format("20060102")
	nowFormat := now.UTC().Format("20060102T150405Z")

	headers.Set(HeaderAliDate, nowFormat)
	headers.Set(HeaderAliContentSHA256, UnsignedPayloadHash)
	headers.Set(httpu.HeaderDate, now.Format(http.TimeFormat))

	for key := range query {
		sort.Strings(query[key])
	}

	sanitizeHostForHeader(req)

	credentialScope := aliBuildScope(nowShortFormat, region, productOSS)
	credentialStr := ak + "/" + credentialScope

	canonicalString := aliCalcCanonicalRequest(req, nil)

	strToSign := aliBuildStringToSign(nowFormat, credentialScope, canonicalString)
	signingSignature := aliCalcSignature(sk, nowShortFormat, region, productOSS, strToSign)

	headers.Set(AuthorizationHeader, aliBuildAuthorizationHeader(credentialStr, signingSignature))
	return nil
}

func aliBuildStringToSign(timeFormat, credentialScope, canonicalRequestString string) string {
	return strings.Join([]string{
		"OSS4-HMAC-SHA256",
		timeFormat,
		credentialScope,
		hex.EncodeToString(makeHash(sha256.New(), []byte(canonicalRequestString))),
	}, "\n")
}

func aliBuildScope(signTime, region, service string) string {
	return strings.Join([]string{
		signTime,
		region,
		service,
		"aliyun_v4_request",
	}, "/")
}

func aliBuildAuthorizationHeader(credentialStr string, signature string) string {
	return fmt.Sprintf("OSS4-HMAC-SHA256 Credential=%s,Signature=%s", credentialStr, signature)
}

func aliIsDefaultSignedHeader(low string) bool {
	if strings.HasPrefix(low, HeaderAliOSSPrefix) ||
		low == "content-type" ||
		low == "content-md5" {
		return true
	}
	return false
}

func aliCalcCanonicalRequest(req *http.Request, additionalHeaders []string) string {
	/*
		Canonical Request
		HTTP Verb + "\n" +
		Canonical URI + "\n" +
		Canonical Query String + "\n" +
		Canonical Headers + "\n" +
		Additional Headers + "\n" +
		Hashed PayLoad
	*/

	// Canonical Uri
	_, bucket, _ := ParseAliOSSURL(req.URL.String())
	canonicalURI := escapePath("/"+bucket+getURIPath(req.URL), false)

	// Canonical Query
	query := strings.ReplaceAll(req.URL.RawQuery, "+", "%20")
	values := make(map[string]string)
	var params []string
	for query != "" {
		var key string
		key, query, _ = strings.Cut(query, "&")
		if key == "" {
			continue
		}
		key, value, _ := strings.Cut(key, "=")
		values[key] = value
		params = append(params, key)
	}
	sort.Strings(params)
	var buf strings.Builder
	for _, k := range params {
		if buf.Len() > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		if len(values[k]) > 0 {
			buf.WriteByte('=')
			buf.WriteString(values[k])
		}
	}
	canonicalQuery := buf.String()

	// Canonical Headers
	var headers []string
	buf.Reset()
	addHeadersMap := make(map[string]bool)
	for _, k := range additionalHeaders {
		addHeadersMap[strings.ToLower(k)] = true
	}
	for k := range req.Header {
		lowk := strings.ToLower(k)
		if aliIsDefaultSignedHeader(lowk) {
			headers = append(headers, lowk)
		} else if _, ok := addHeadersMap[lowk]; ok {
			headers = append(headers, lowk)
		}
	}
	sort.Strings(headers)
	for _, k := range headers {
		headerValues := make([]string, len(req.Header.Values(k)))
		for i, v := range req.Header.Values(k) {
			headerValues[i] = strings.TrimSpace(v)
		}
		buf.WriteString(k)
		buf.WriteString(":")
		buf.WriteString(strings.Join(headerValues, ","))
		buf.WriteString("\n")
	}
	canonicalHeaders := buf.String()

	// Additional Headers
	canonicalAdditionalHeaders := strings.Join(additionalHeaders, ";")

	hashPayload := UnsignedPayloadHash
	if val := req.Header.Get(HeaderAliContentSHA256); val != "" {
		hashPayload = val
	}

	buf.Reset()
	buf.WriteString(req.Method)
	buf.WriteString("\n")
	buf.WriteString(canonicalURI)
	buf.WriteString("\n")
	buf.WriteString(canonicalQuery)
	buf.WriteString("\n")
	buf.WriteString(canonicalHeaders)
	buf.WriteString("\n")
	buf.WriteString(canonicalAdditionalHeaders)
	buf.WriteString("\n")
	buf.WriteString(hashPayload)

	return buf.String()
}

func aliCalcSignature(sk, date, region, product, stringToSign string) string {
	hmacHash := sha256.New

	signingKey := "aliyun_v4" + sk

	h1 := hmac.New(hmacHash, []byte(signingKey))
	io.WriteString(h1, date)
	h1Key := h1.Sum(nil)

	h2 := hmac.New(hmacHash, h1Key)
	io.WriteString(h2, region)
	h2Key := h2.Sum(nil)

	h3 := hmac.New(hmacHash, h2Key)
	io.WriteString(h3, product)
	h3Key := h3.Sum(nil)

	h4 := hmac.New(hmacHash, h3Key)
	io.WriteString(h4, "aliyun_v4_request")
	h4Key := h4.Sum(nil)

	h := hmac.New(hmacHash, h4Key)
	io.WriteString(h, stringToSign)
	signature := hex.EncodeToString(h.Sum(nil))

	return signature
}
