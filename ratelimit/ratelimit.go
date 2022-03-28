package ratelimit

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/tidwall/gjson"
)

type RateLimiterType string

type RxTxRateLimiter interface {
	runner.Runner
	plugin.Plugin[RateLimiterType]
	RxWaitN(n int, timeout int) error
	TxWaitN(n int, timeout int) error
}

type RateLimiterCfg struct {
	Type RateLimiterType `json:"type" yaml:"type"`
	Cfg  any             `json:"cfg"  yaml:"cfg"`
}

type rateLimiterCfgOnlyCfg struct {
	Cfg any `json:"cfg" yaml:"cfg"`
}

func (c *RateLimiterCfg) UnmarshalJSON(data []byte) error {
	return c.customUnmarshal(data, jsons.Unmarshal)
}

func (c *RateLimiterCfg) UnmarshalYAML(data []byte) error {
	return c.customUnmarshal(data, yamls.Unmarshal)
}

func (c *RateLimiterCfg) customUnmarshal(data []byte, unmarshaler func([]byte, any) error) error {
	typ := gjson.GetBytes(data, "type")
	if !typ.Exists() {
		return errs.Errorf("ratelimiter type is not present")
	}
	if typ.Type != gjson.String {
		return errs.Errorf("invalid ratelimiter type")
	}
	c.Type = RateLimiterType(typ.Str)

	cv := rateLimiterCfgOnlyCfg{}
	cv.Cfg = plugin.CreateCfg[RateLimiterType, any](c.Type)
	if cv.Cfg == nil {
		return errs.Errorf("created rw cfg is nil: %s", c.Type)
	}
	err := unmarshaler(data, &cv)
	if err != nil {
		return err
	}
	c.Cfg = cv.Cfg
	return nil
}
