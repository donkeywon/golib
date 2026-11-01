package ratelimit

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/tidwall/gjson"
)

type RxTxRateLimiter interface {
	Init(ctx context.Context) error
	RxWaitN(ctx context.Context, n int, timeout time.Duration) error
	TxWaitN(ctx context.Context, n int, timeout time.Duration) error
}

type Type string

type Cfg struct {
	Type Type `json:"type" yaml:"type"`
	Cfg  any  `json:"cfg"  yaml:"cfg"`
}

type rateLimiterCfgOnlyCfg struct {
	Cfg any `json:"cfg" yaml:"cfg"`
}

func (c *Cfg) UnmarshalJSON(data []byte) error {
	typ := gjson.GetBytes(data, "type")
	if !typ.Exists() {
		return errs.Errorf("empty ratelimiter type")
	}
	if typ.Type != gjson.String {
		return errs.Errorf("invalid ratelimiter type")
	}
	c.Type = Type(typ.Str)

	return c.customUnmarshal(data, func(b []byte, a any) error { return json.Unmarshal(b, a) })
}

func (c *Cfg) UnmarshalYAML(data []byte) error {
	yp, _ := yaml.PathString("$.type")
	node, err := yp.ReadNode(bytes.NewReader(data))
	if err != nil {
		return errs.Wrapf(err, "get ratelimiter type failed")
	}
	if node.Type() != ast.StringType {
		return errs.Errorf("invalid ratelimiter type")
	}
	c.Type = Type(node.String())

	return c.customUnmarshal(data, yaml.Unmarshal)
}

func (c *Cfg) customUnmarshal(data []byte, unmarshaler func([]byte, any) error) error {
	cv := rateLimiterCfgOnlyCfg{}
	cv.Cfg = plugin.CreateCfg[any](c.Type)
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
