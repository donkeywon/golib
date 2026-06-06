package step

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/tidwall/gjson"
)

type Type string

type Cfg struct {
	Type Type `json:"type" validate:"required" yaml:"type"`
	Cfg  any  `json:"cfg"  validate:"required" yaml:"cfg"`
}

type stepCfgOnlyCfg struct {
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
		return errs.Errorf("step type is not present")
	}
	if typ.Type != gjson.String {
		return errs.Errorf("invalid step type")
	}
	c.Type = Type(typ.Str)

	cv := stepCfgOnlyCfg{}
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

type Step interface {
	kvs.KVS[string, any]
	runner.Runner
}

type Base struct {
	kvs.Map[string, any]
	runner.Base
}
