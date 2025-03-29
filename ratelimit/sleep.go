package ratelimit

import (
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

func init() {
	plugin.RegWithCfg(TypeSleep, func() RxTxRateLimiter { return NewSleepRateLimiter() }, func() any { return NewSleepRateLimiterCfg() })
}

const TypeSleep Type = "sleep"

type SleepRateLimiterCfg struct {
	Millisecond int
}

func NewSleepRateLimiterCfg() *SleepRateLimiterCfg {
	return &SleepRateLimiterCfg{}
}

type SleepRateLimiter struct {
	runner.Runner
	*SleepRateLimiterCfg
}

func NewSleepRateLimiter() *SleepRateLimiter {
	return &SleepRateLimiter{
		Runner:              runner.Create("sleepRateLimiter"),
		SleepRateLimiterCfg: NewSleepRateLimiterCfg(),
	}
}

func (srl *SleepRateLimiter) Init() error {
	if srl.SleepRateLimiterCfg.Millisecond <= 0 {
		return errs.Errorf("sleep rate limiter Millisecond must gt 0: %d", srl.SleepRateLimiterCfg.Millisecond)
	}
	return srl.Runner.Init()
}

func (srl *SleepRateLimiter) waitN(n int, timeout int) error {
	if n == 0 {
		return nil
	}

	sn := srl.Millisecond
	if timeout > 0 && timeout < srl.Millisecond*1000 {
		sn = timeout
	}
	time.Sleep(time.Millisecond * time.Duration(sn))
	return nil
}

func (srl *SleepRateLimiter) RxWaitN(n int, timeout int) error {
	return srl.waitN(n, timeout)
}

func (srl *SleepRateLimiter) TxWaitN(n int, timeout int) error {
	return srl.waitN(n, timeout)
}

func (srl *SleepRateLimiter) Type() Type {
	return TypeSleep
}
