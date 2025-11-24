package runner

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/donkeywon/golib/errs"
)

type runA struct {
	Runner
}

func (ra *runA) Init() error {
	return ra.Runner.Init()
}

func (ra *runA) Start() error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := 0; i < 5; i++ {
		select {
		case <-ra.Ctx().Done():
			return context.Canceled
		case <-ra.Stopping():
			return nil
		case <-t.C:
			ra.Info(strconv.Itoa(i))
		}
	}
	return nil
}

func (ra *runA) Stop() error {
	return nil
}

type runB struct {
	Runner
}

func (rb *runB) Init() error {
	return rb.Runner.Init()
}

func (rb *runB) Start() error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := 0; i < 5; i++ {
		select {
		case <-rb.Stopping():
			return nil
		case <-t.C:
			rb.Info(strconv.Itoa(i))
		}
	}
	return nil
}

func (rb *runB) Stop() error {
	return nil
}

type runC struct {
	Runner
}

func (rc *runC) Init() error {
	return rc.Runner.Init()
}

func (rc *runC) Start() error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := 0; i < 5; i++ {
		select {
		case <-rc.Stopping():
			return nil
		case <-t.C:
			rc.Info(strconv.Itoa(i))
		}
	}
	return nil
}

func (rc *runC) Stop() error {
	return nil
}

var ra = &runA{
	Runner: newBase("runA"),
}

var rb = &runB{
	Runner: newBase("runB"),
}

var rc = &runC{
	Runner: newBase("runC"),
}

func init() {
	ra.SetLogLevel("debug")
}

func TestSimpleRun(t *testing.T) {
	ra.SetCtx(t.Context())
	Init(ra)
	err := Run(ra)
	if err != nil {
		println(errs.ErrToStackString(err))
	}
}

func TestStopBeforeStart(_ *testing.T) {
	_ = Init(ra)
	Stop(ra)
	Start(ra)
	<-ra.Done()
}
