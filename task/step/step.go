package step

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/tidwall/gjson"
)

var CreateBase = newBase

type Type string

type Cfg struct {
	Type Type `json:"type" validate:"required" yaml:"type"`
	Cfg  any  `json:"cfg"  validate:"required" yaml:"cfg"`
}

type stepCfgOnlyCfg struct {
	Cfg any `json:"cfg" yaml:"cfg"`
}

func (s *Cfg) UnmarshalJSON(data []byte) error {
	return s.customUnmarshal(data, jsons.Unmarshal)
}

func (s *Cfg) UnmarshalYAML(data []byte) error {
	return s.customUnmarshal(data, yamls.Unmarshal)
}

func (s *Cfg) customUnmarshal(data []byte, unmarshaler func([]byte, any) error) error {
	typ := gjson.GetBytes(data, "type")
	if !typ.Exists() {
		return errs.Errorf("step type is not present")
	}
	if typ.Type != gjson.String {
		return errs.Errorf("invalid step type")
	}
	s.Type = Type(typ.Str)

	cv := stepCfgOnlyCfg{}
	cv.Cfg = plugin.CreateCfg(s.Type)
	if cv.Cfg == nil {
		return errs.Errorf("unknown type: %s", typ.Str)
	}
	err := unmarshaler(data, &cv)
	if err != nil {
		return err
	}
	s.Cfg = cv.Cfg
	return nil
}

type Step interface {
	runner.Runner
	plugin.Plugin
}

type baseStep struct {
	runner.Runner
}

func newBase(name string) Step {
	return &baseStep{
		Runner: runner.Create(name),
	}
}

func (b *baseStep) Store(k string, v any) {
	b.Runner.StoreAsString(k, v)
}
