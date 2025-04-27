package oss

import (
	"bytes"
	"github.com/donkeywon/golib/util/conv"
	"strconv"
)

type Cfg struct {
	URL      string `json:"url"            yaml:"url"     validate:"required"`
	Ak       string `json:"ak"             yaml:"ak"`
	Sk       string `json:"sk"             yaml:"sk"`
	Region   string `json:"region"         yaml:"region"`
	Retry    int    `json:"retry"          yaml:"retry"   validate:"gte=1"`
	Timeout  int    `json:"timeout"        yaml:"timeout" validate:"gte=1"`
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
		c.PartSize = 8 * 1024 * 1024
	}
	if c.Parallel <= 0 {
		c.Parallel = 1
	}
}

func (c Cfg) MarshalJSON() (ret []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, 192))
	buf.WriteByte('{')
	buf.WriteString(`"url":`)
	buf.WriteString(strconv.Quote(c.URL))
	buf.WriteByte(',')
	buf.WriteString(`"region":`)
	buf.WriteString(strconv.Quote(c.Region))
	buf.WriteByte(',')
	buf.WriteString(`"retry":`)
	buf.WriteString(strconv.Itoa(c.Retry))
	buf.WriteByte(',')
	buf.WriteString(`"timeout":`)
	buf.WriteString(strconv.Itoa(c.Timeout))
	buf.WriteByte(',')
	buf.WriteString(`"offset":`)
	buf.WriteString(strconv.FormatInt(c.Offset, 10))
	buf.WriteByte(',')
	buf.WriteString(`"partSize":`)
	buf.WriteString(strconv.FormatInt(c.PartSize, 10))
	buf.WriteByte(',')
	buf.WriteString(`"parallel":`)
	buf.WriteString(strconv.Itoa(c.Parallel))
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (c *Cfg) String() string {
	b, _ := c.MarshalJSON()
	return conv.Bytes2String(b)
}
