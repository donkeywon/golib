package oss

type Cfg struct {
	URL      string `json:"url"            yaml:"url"     validate:"required"`
	Retry    int    `json:"retry"          yaml:"retry"   validate:"gte=1"`
	Timeout  int    `json:"timeout"        yaml:"timeout" validate:"gte=1"`
	Ak       string `json:"ak"             yaml:"ak"`
	Sk       string `json:"sk"             yaml:"sk"`
	Region   string `json:"region"         yaml:"region"`
	Offset   int64  `json:"offset"         yaml:"offset"`
	PartSize int64  `json:"partSize"       yaml:"partSize"`
	NoRange  bool   `json:"noRange"        yaml:"noRange"`
}

func (c *Cfg) setDefaults() {
	if c.Retry <= 0 {
		c.Retry = 1
	}
	if c.Timeout <= 0 {
		c.Timeout = 60
	}
	if c.PartSize <= 0 {
		c.PartSize = 8 * 1024 * 1024
	}
}
