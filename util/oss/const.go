package oss

const (
	HeaderAuthorization     = "Authorization"
	HeaderDate              = "Date"
	HeaderContentLength     = "Content-Length"
	HeaderContentEncoding   = "Content-Encoding"
	HeaderContentLanguage   = "Content-Language"
	HeaderContentType       = "Content-Type"
	HeaderContentMD5        = "Content-MD5"
	HeaderIfModifiedSince   = "If-Modified-Since"
	HeaderIfMatch           = "If-Match"
	HeaderIfNoneMatch       = "If-None-Match"
	HeaderIfUnmodifiedSince = "If-Unmodified-Since"
	HeaderRange             = "Range"

	HeaderXmsDate      = "x-ms-date"
	HeaderXmsVersion   = "x-ms-version"
	HeaderXmsRequestID = "x-ms-request-id"
	HeaderXmsBlobType  = "x-ms-blob-type"

	HeaderOssAppendNextPositionHeader    = "X-Rgw-Next-Append-Position"
	HeaderObsAppendNextPositionHeader    = "X-Obs-Next-Append-Position"
	HeaderAliOssAppendNextPositionHeader = "X-Oss-Next-Append-Position"
	HeaderAzblobAppendOffsetHeader       = "x-ms-blob-append-offset"
	HeaderAzblobAppendPositionHeader     = "x-ms-blob-condition-appendpos"

	HeaderAmzDate          = "X-Amz-Date"
	HeaderAmzContentSHA256 = "X-Amz-Content-Sha256"

	HeaderAliOssPrefix     = "x-oss-"
	HeaderAliContentSHA256 = "X-Oss-Content-Sha256"
	HeaderAliDate          = "X-Oss-Date"

	AmzServiceS3          = "s3"
	UnsignedPayloadHash   = "UNSIGNED-PAYLOAD"
	AmzS3SigningAlgorithm = "AWS4-HMAC-SHA256"
	AuthorizationHeader   = "Authorization"
)

type Type string

const (
	OssTypeUnknown Type = "unknown"
	OssTypeAmz     Type = "amz"
	OssTypeBlob    Type = "blob"
	OssTypeObs     Type = "obs"
	OssTypeAliOss  Type = "alioss"
)

var (
	AmzSignIgnoredHeaders = map[string]struct{}{
		"Authorization":   {},
		"User-Agent":      {},
		"X-Amzn-Trace-Id": {},
		"Expect":          {},
	}
)
