package httpio

type Cfg struct {
	BeginPos int64 `json:"beginPos" yaml:"beginPos"`
	EndPos   int64 `json:"endPos"   yaml:"endPos"`
	PartSize int64 `json:"partSize" yaml:"partSize"`
	Retry    int   `json:"retry"    yaml:"retry"`
}
