package oss

type Cfg struct {
	URL     string `json:"url"     validate:"min=1" yaml:"url"`
	Ak      string `json:"ak"      validate:"min=1" yaml:"ak"`
	Sk      string `json:"sk"      validate:"min=1" yaml:"sk"`
	Retry   int    `json:"retry"   validate:"gte=1" yaml:"retry"`
	Timeout int    `json:"timeout" validate:"gte=1" yaml:"timeout"`
	Region  string `json:"region"  yaml:"region"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}
