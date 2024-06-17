package oss

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var amzS3URLRegex = regexp.MustCompile(`\.?s3\.([^\.]+)\.amazonaws\.com`)

const amzS3URLMatchCount = 2

var noEscape [256]bool

func init() {
	for i := 0; i < len(noEscape); i++ {
		// AWS expects every character except these to be escaped
		noEscape[i] = (i >= 'A' && i <= 'Z') ||
			(i >= 'a' && i <= 'z') ||
			(i >= '0' && i <= '9') ||
			i == '-' ||
			i == '.' ||
			i == '_' ||
			i == '~'
	}
}

func IsAmzS3(url string) bool {
	ok, _ := ParseAmzS3Url(url)
	return ok
}

func ParseAmzS3Url(url string) (bool, string) {
	matchRes := amzS3URLRegex.FindStringSubmatch(url)
	if len(matchRes) != amzS3URLMatchCount {
		return false, ""
	}
	return true, matchRes[1]
}

func AmzSign(req *http.Request, ak string, sk string, region string) error {
	query := req.URL.Query()
	headers := req.Header

	now := time.Now()
	nowShortFormat := now.UTC().Format("20060102")
	nowFormat := now.UTC().Format("20060102T150405Z")

	headers.Set(HeaderAmzDate, nowFormat)
	headers.Set(HeaderAmzContentSHA256, UnsignedPayloadHash)

	for key := range query {
		sort.Strings(query[key])
	}

	sanitizeHostForHeader(req)

	credentialScope := buildCredentialScope(nowShortFormat, region, AmzServiceS3)
	credentialStr := ak + "/" + credentialScope

	unsignedHeaders := headers
	host := req.URL.Host
	if len(req.Host) > 0 {
		host = req.Host
	}

	_, signedHeadersStr, canonicalHeaderStr := buildCanonicalHeaders(host, unsignedHeaders, req.ContentLength)

	var rawQuery strings.Builder
	rawQuery.WriteString(strings.ReplaceAll(query.Encode(), "+", "%20"))

	canonicalURI := escapePath(getURIPath(req.URL), false)

	canonicalString := buildCanonicalString(
		req.Method,
		canonicalURI,
		rawQuery.String(),
		canonicalHeaderStr,
		signedHeadersStr,
	)

	strToSign := buildStringToSign(nowFormat, credentialScope, canonicalString)
	signingSignature := buildSignature(sk, AmzServiceS3, region, strToSign, nowShortFormat)

	headers.Set(AuthorizationHeader, buildAuthorizationHeader(credentialStr, signedHeadersStr, signingSignature))
	return nil
}

// below copied from github.com/aws/aws-sdk-go-v2

func buildAuthorizationHeader(credentialStr, signedHeadersStr, signingSignature string) string {
	const credential = "Credential" + "="
	const signedHeaders = "SignedHeaders="
	const signature = "Signature="
	const commaSpace = ", "

	var parts strings.Builder
	parts.Grow(len(AmzS3SigningAlgorithm) + 1 +
		len(credential) + len(credentialStr) + 2 +
		len(signedHeaders) + len(signedHeadersStr) + 2 +
		len(signature) + len(signingSignature),
	)
	parts.WriteString(AmzS3SigningAlgorithm)
	parts.WriteRune(' ')
	parts.WriteString(credential)
	parts.WriteString(credentialStr)
	parts.WriteString(commaSpace)
	parts.WriteString(signedHeaders)
	parts.WriteString(signedHeadersStr)
	parts.WriteString(commaSpace)
	parts.WriteString(signature)
	parts.WriteString(signingSignature)
	return parts.String()
}

// HMACSHA256 computes a HMAC-SHA256 of data given the provided key.
func HMACSHA256(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

func deriveKey(secret, service, region, timeShortFormat string) []byte {
	hmacDate := HMACSHA256([]byte("AWS4"+secret), []byte(timeShortFormat))
	hmacRegion := HMACSHA256(hmacDate, []byte(region))
	hmacService := HMACSHA256(hmacRegion, []byte(service))
	return HMACSHA256(hmacService, []byte("aws4_request"))
}

func buildSignature(sk, service, region, strToSign, timeShortFormat string) string {
	key := deriveKey(sk, service, region, timeShortFormat)
	return hex.EncodeToString(HMACSHA256(key, []byte(strToSign)))
}

func makeHash(hash hash.Hash, b []byte) []byte {
	hash.Reset()
	hash.Write(b)
	return hash.Sum(nil)
}

func buildStringToSign(timeFormat, credentialScope, canonicalRequestString string) string {
	return strings.Join([]string{
		AmzS3SigningAlgorithm,
		timeFormat,
		credentialScope,
		hex.EncodeToString(makeHash(sha256.New(), []byte(canonicalRequestString))),
	}, "\n")
}

func buildCanonicalString(method, uri, query, canonicalHeaders, signedHeaders string) string {
	return strings.Join([]string{
		method,
		uri,
		query,
		canonicalHeaders,
		signedHeaders,
		UnsignedPayloadHash,
	}, "\n")
}

// EscapePath escapes part of a URL path in Amazon style.
func escapePath(path string, encodeSep bool) string {
	var buf bytes.Buffer
	for i := range len(path) {
		c := path[i]
		if noEscape[c] || (c == '/' && !encodeSep) {
			buf.WriteByte(c)
		} else {
			fmt.Fprintf(&buf, "%%%02X", c)
		}
	}
	return buf.String()
}

func buildCanonicalHeaders(host string, header http.Header, length int64) (
	http.Header, string, string,
) {
	signed := make(http.Header)

	var headers []string
	const hostHeader = "host"
	headers = append(headers, hostHeader)
	signed[hostHeader] = append(signed[hostHeader], host)

	const contentLengthHeader = "content-length"
	if length > 0 {
		headers = append(headers, contentLengthHeader)
		signed[contentLengthHeader] = append(signed[contentLengthHeader], strconv.FormatInt(length, 10))
	}

	for k, v := range header {
		if _, ok := AmzSignIgnoredHeaders[k]; ok {
			continue
		}
		if strings.EqualFold(k, contentLengthHeader) {
			// prevent signing already handled content-length header.
			continue
		}

		lowerCaseKey := strings.ToLower(k)
		if _, ok := signed[lowerCaseKey]; ok {
			// include additional values
			signed[lowerCaseKey] = append(signed[lowerCaseKey], v...)
			continue
		}

		headers = append(headers, lowerCaseKey)
		signed[lowerCaseKey] = v
	}
	sort.Strings(headers)

	signedHeaders := strings.Join(headers, ";")

	var canonicalHeaders strings.Builder
	n := len(headers)
	const colon = ':'
	for i := range n {
		if headers[i] == hostHeader {
			canonicalHeaders.WriteString(hostHeader)
			canonicalHeaders.WriteRune(colon)
			canonicalHeaders.WriteString(stripExcessSpaces(host))
		} else {
			canonicalHeaders.WriteString(headers[i])
			canonicalHeaders.WriteRune(colon)
			// Trim out leading, trailing, and dedup inner spaces from signed header values.
			values := signed[headers[i]]
			for j, v := range values {
				cleanedValue := strings.TrimSpace(stripExcessSpaces(v))
				canonicalHeaders.WriteString(cleanedValue)
				if j < len(values)-1 {
					canonicalHeaders.WriteRune(',')
				}
			}
		}
		canonicalHeaders.WriteRune('\n')
	}
	canonicalHeadersStr := canonicalHeaders.String()

	return signed, signedHeaders, canonicalHeadersStr
}

// GetURIPath returns the escaped URI component from the provided URL.
func getURIPath(u *url.URL) string {
	var uriPath string

	if len(u.Opaque) > 0 {
		const schemeSep, pathSep, queryStart = "//", "/", "?"

		opaque := u.Opaque
		// Cut off the query string if present.
		if idx := strings.Index(opaque, queryStart); idx >= 0 {
			opaque = opaque[:idx]
		}

		// Cutout the scheme separator if present.
		if strings.Index(opaque, schemeSep) == 0 {
			opaque = opaque[len(schemeSep):]
		}

		// capture URI path starting with first path separator.
		if idx := strings.Index(opaque, pathSep); idx >= 0 {
			uriPath = opaque[idx:]
		}
	} else {
		uriPath = u.EscapedPath()
	}

	if len(uriPath) == 0 {
		uriPath = "/"
	}

	return uriPath
}

// StripExcessSpaces will rewrite the passed in slice's string values to not
// contain multiple side-by-side spaces.
func stripExcessSpaces(str string) string {
	var j, k, l, m, spaces int
	// Trim trailing spaces
	for j = len(str) - 1; j >= 0; j-- {
		if str[j] != ' ' {
			break
		}
	}

	// Trim leading spaces
	for k = range j {
		if str[k] != ' ' {
			break
		}
	}
	str = str[k : j+1]

	// Strip multiple spaces.
	j = strings.Index(str, "  ")
	if j < 0 {
		return str
	}

	buf := []byte(str)
	for k, m, l = j, j, len(buf); k < l; k++ {
		if buf[k] == ' ' {
			if spaces == 0 {
				// First space.
				buf[m] = buf[k]
				m++
			}
			spaces++
		} else {
			// End of multiple spaces.
			spaces = 0
			buf[m] = buf[k]
			m++
		}
	}

	return string(buf[:m])
}

func buildCredentialScope(signTime string, region, service string) string {
	return strings.Join([]string{
		signTime,
		region,
		service,
		"aws4_request",
	}, "/")
}

// SanitizeHostForHeader removes default port from host and updates request.Host.
func sanitizeHostForHeader(r *http.Request) {
	host := getHost(r)
	port := portOnly(host)
	if port != "" && isDefaultPort(r.URL.Scheme, port) {
		r.Host = stripPort(host)
	}
}

// Returns host from request.
func getHost(r *http.Request) string {
	if r.Host != "" {
		return r.Host
	}

	return r.URL.Host
}

// Hostname returns u.Host, without any port number.
//
// If Host is an IPv6 literal with a port number, Hostname returns the
// IPv6 literal without the square brackets. IPv6 literals may include
// a zone identifier.
//
// Copied from the Go 1.8 standard library (net/url).
func stripPort(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}

// Port returns the port part of u.Host, without the leading colon.
// If u.Host doesn't contain a port, Port returns an empty string.
//
// Copied from the Go 1.8 standard library (net/url).
func portOnly(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return ""
	}
	if i := strings.Index(hostport, "]:"); i != -1 {
		return hostport[i+len("]:"):]
	}
	if strings.Contains(hostport, "]") {
		return ""
	}
	return hostport[colon+len(":"):]
}

// Returns true if the specified URI is using the standard port
// (i.e. port 80 for HTTP URIs or 443 for HTTPS URIs).
func isDefaultPort(scheme, port string) bool {
	if port == "" {
		return true
	}

	lowerCaseScheme := strings.ToLower(scheme)
	if (lowerCaseScheme == "http" && port == "80") || (lowerCaseScheme == "https" && port == "443") {
		return true
	}

	return false
}
