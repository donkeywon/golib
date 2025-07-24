package runner

import (
	"strconv"
	"testing"
	"time"
)

type runA struct {
	Runner
}

func (ra *runA) Init() error {
	return nil
}

func (ra *runA) Start() error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := 0; i < 5; i++ {
		select {
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
	return nil
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
	return nil
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

func TestSimpleRun(_ *testing.T) {
	Start(ra)
}

func TestStopBeforeStart(_ *testing.T) {
	_ = Init(ra)
	Stop(ra)
	Start(ra)
	<-ra.Done()
}
