package runner

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/donkeywon/golib/log"
	"github.com/stretchr/testify/require"
)

var l, _ = log.NewCfg().Build()

type runA struct {
	*BaseRunner
}

func (ra *runA) Init() error {
	return nil
}

func (ra *runA) Start() error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := range 5 {
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
	*BaseRunner
}

func (rb *runB) Init() error {
	return nil
}

func (rb *runB) Start() error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := range 5 {
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
	*BaseRunner
}

func (rc *runC) Init() error {
	return nil
}

func (rc *runC) Start() error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := range 5 {
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
	BaseRunner: NewBase("runA"),
}

var rb = &runB{
	BaseRunner: NewBase("runB"),
}

var rc = &runC{
	BaseRunner: NewBase("runC"),
}

func init() {
	ra.WithLogger(l)
}

func TestSimpleRun(_ *testing.T) {
	Start(ra)
	<-ra.Done()
}

func TestSimpleRunBG(t *testing.T) {
	ra.AppendRunner(rb)
	ra.AppendRunner(rc)

	require.NoError(t, Init(ra))

	ra.Store("abc", true)

	StartBG(ra)

	// time.Sleep(time.Second * 2)
	// Stop(ra)
	<-ra.Done()
}

func TestRunBGWithChildrenCancel(t *testing.T) {
	ra.AppendRunner(rb)
	ra.AppendRunner(rc)

	ctx, cancel := context.WithCancel(context.Background())
	ra.SetCtx(ctx)

	require.NoError(t, Init(ra))

	StartBG(ra)

	go func() {
		time.Sleep(time.Second * 2)
		cancel()
	}()
	<-ra.Done()
}

func TestStopBeforeStart(_ *testing.T) {
	_ = Init(ra)
	Stop(ra)
	Start(ra)
	<-ra.Done()
}