package task

import (
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

var CreateBaseStep = newBase

type StepType string

type Step interface {
	runner.Runner
	plugin.Plugin
	GetTask() *Task
	SetTask(*Task)
}

type baseStep struct {
	runner.Runner

	t *Task
}

func newBase(name string) Step {
	return &baseStep{
		Runner: runner.Create(name),
	}
}

func (b *baseStep) Store(k string, v any) {
	b.Runner.StoreAsString(k, v)
}

func (b *baseStep) GetTask() *Task {
	return b.t
}

func (b *baseStep) SetTask(t *Task) {
	b.t = t
}

func (b *baseStep) Type() any {
	panic("method Step.Type not implemented")
}

func (b *baseStep) GetCfg() any {
	panic("method Step.GetCfg not implemented")
}
