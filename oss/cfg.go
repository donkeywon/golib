package oss

type Cfg struct {
	URL     string `json:"url"     validate:"min=1" yaml:"url"`
	Retry   int    `json:"retry"   validate:"gte=1" yaml:"retry"`
	Timeout int    `json:"timeout" validate:"gte=1" yaml:"timeout"`
	Ak      string `json:"ak"      yaml:"ak"`
	Sk      string `json:"sk"      yaml:"sk"`
	Region  string `json:"region"  yaml:"region"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}
