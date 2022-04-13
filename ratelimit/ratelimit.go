package ratelimit

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/tidwall/gjson"
)

type Type string

type RxTxRateLimiter interface {
	runner.Runner
	plugin.Plugin
	RxWaitN(n int, timeout int) error
	TxWaitN(n int, timeout int) error
}

type Cfg struct {
	Type Type `json:"type" yaml:"type"`
	Cfg  any  `json:"cfg"  yaml:"cfg"`
}

type rateLimiterCfgOnlyCfg struct {
	Cfg any `json:"cfg" yaml:"cfg"`
}

func (c *Cfg) UnmarshalJSON(data []byte) error {
	return c.customUnmarshal(data, jsons.Unmarshal)
}

func (c *Cfg) UnmarshalYAML(data []byte) error {
	return c.customUnmarshal(data, yamls.Unmarshal)
}

func (c *Cfg) customUnmarshal(data []byte, unmarshaler func([]byte, any) error) error {
	typ := gjson.GetBytes(data, "type")
	if !typ.Exists() {
		return errs.Errorf("ratelimiter type is not present")
	}
	if typ.Type != gjson.String {
		return errs.Errorf("invalid ratelimiter type")
	}
	c.Type = Type(typ.Str)

	cv := rateLimiterCfgOnlyCfg{}
	cv.Cfg = plugin.CreateCfg(c.Type)
	if cv.Cfg == nil {
		return nil
	}
	err := unmarshaler(data, &cv)
	if err != nil {
		return err
	}
	c.Cfg = cv.Cfg
	return nil
}
