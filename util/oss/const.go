package oss

const (
	HeaderXmsDate      = "X-Ms-Date"
	HeaderXmsVersion   = "X-Ms-Version"
	HeaderXmsRequestID = "X-Ms-Request-Id"
	HeaderXmsBlobType  = "X-Ms-Blob-Type"

	HeaderOSSAppendNextPositionHeader    = "X-Rgw-Next-Append-Position"
	HeaderOBSAppendNextPositionHeader    = "X-Obs-Next-Append-Position"
	HeaderAliOSSAppendNextPositionHeader = "X-Oss-Next-Append-Position"
	HeaderAzblobAppendOffsetHeader       = "X-Ms-Blob-Append-Offset"
	HeaderAzblobAppendPositionHeader     = "X-Ms-Blob-Condition-Appendpos"

	HeaderAmzDate          = "X-Amz-Date"
	HeaderAmzContentSHA256 = "X-Amz-Content-Sha256"

	HeaderAliOSSPrefix     = "x-oss-"
	HeaderAliContentSHA256 = "X-Oss-Content-Sha256"
	HeaderAliDate          = "X-Oss-Date"

	AmzServiceS3          = "s3"
	UnsignedPayloadHash   = "UNSIGNED-PAYLOAD"
	AmzS3SigningAlgorithm = "AWS4-HMAC-SHA256"
	AuthorizationHeader   = "Authorization"
)

type Type string

const (
	TypeUnknown   Type = "unknown"
	TypeAmazonS3  Type = "AmazonS3"
	TypeBlob      Type = "Blob"
	TypeOBS       Type = "OBS"
	TypeAliyunOSS Type = "AliyunOSS"
	TypeMinIO     Type = "MinIO"
)

var (
	AmzSignIgnoredHeaders = map[string]struct{}{
		"Authorization":   {},
		"User-Agent":      {},
		"X-Amzn-Trace-Id": {},
		"Expect":          {},
	}
)
