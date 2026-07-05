package oss

import "time"

type Cfg struct {
	URL      string        `json:"url"            yaml:"url"     validate:"required"`
	Ak       string        `json:"ak"             yaml:"ak"`
	Sk       string        `json:"sk"             yaml:"sk"`
	Region   string        `json:"region"         yaml:"region"`
	Retry    int           `json:"retry"          yaml:"retry"`
	Timeout  time.Duration `json:"timeout"        yaml:"timeout"`  // for reader is Head Timeout and ResponseHeaderTimeout, for writer is init and put and abort
	Offset   int64         `json:"offset"         yaml:"offset"`   // for reader and append writer
	PartSize int64         `json:"partSize"       yaml:"partSize"` // for writer
	Parallel int           `json:"parallel"       yaml:"parallel"` // only for multipart writer
}

func (c *Cfg) setDefaults() {
	if c.Retry <= 0 {
		c.Retry = 1
	}
	if c.Timeout <= 0 {
		c.Timeout = 60 * time.Second
	}
	if c.PartSize <= 0 {
		c.PartSize = 512 * 1024
	}
	if c.Parallel <= 0 {
		c.Parallel = 1
	}
}
