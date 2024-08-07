package task

import (
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

type StepType string

type Step interface {
	runner.Runner
	plugin.Plugin
	GetTask() *Task
	SetTask(*Task)
	CollectAsString() map[string]string
}

type BaseStep struct {
	runner.Runner

	t *Task
}

func NewBase(name string) Step {
	return &BaseStep{
		Runner: runner.Create(name),
	}
}

func (b *BaseStep) Store(k string, v any) {
	b.Runner.StoreAsString(k, v)
}

func (b *BaseStep) GetTask() *Task {
	return b.t
}

func (b *BaseStep) SetTask(t *Task) {
	b.t = t
}

func (b *BaseStep) Type() interface{} {
	panic("method Step.Type not implemented")
}

func (b *BaseStep) GetCfg() interface{} {
	panic("method Step.GetCfg not implemented")
}
