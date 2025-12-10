package oss

type Cfg struct {
	URL      string `json:"url"            yaml:"url"     validate:"required"`
	Ak       string `json:"ak"             yaml:"ak"`
	Sk       string `json:"sk"             yaml:"sk"`
	Region   string `json:"region"         yaml:"region"`
	Retry    int    `json:"retry"          yaml:"retry"`
	Timeout  int    `json:"timeout"        yaml:"timeout"`
	Offset   int64  `json:"offset"         yaml:"offset"`
	PartSize int64  `json:"partSize"       yaml:"partSize"`
	Parallel int    `json:"parallel"       yaml:"parallel"`
}

func (c *Cfg) setDefaults() {
	if c.Retry <= 0 {
		c.Retry = 1
	}
	if c.Timeout <= 0 {
		c.Timeout = 60
	}
	if c.PartSize <= 0 {
		c.PartSize = 512 * 1024
	}
	if c.Parallel <= 0 {
		c.Parallel = 1
	}
}
