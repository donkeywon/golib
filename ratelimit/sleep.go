package ratelimit

import (
	"context"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

func init() {
	plugin.Reg(TypeSleep, func() RxTxRateLimiter { return NewSleepRateLimiter() }, func() any { return NewSleepRateLimiterCfg() })
}

const TypeSleep Type = "sleep"

type SleepRateLimiterCfg struct {
	Microsecond int
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
	if srl.SleepRateLimiterCfg.Microsecond <= 0 {
		return errs.Errorf("sleep rate limiter Microsecond must gt 0: %d", srl.SleepRateLimiterCfg.Microsecond)
	}
	return srl.Runner.Init()
}

func (srl *SleepRateLimiter) waitN(ctx context.Context, n int, timeout time.Duration) error {
	if n == 0 {
		return nil
	}

	sn := time.Duration(srl.Microsecond) * time.Microsecond
	if timeout > 0 && timeout < sn {
		sn = timeout
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, sn)
	defer cancel()
	<-ctx.Done()
	return nil
}

func (srl *SleepRateLimiter) RxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return srl.waitN(ctx, n, timeout)
}

func (srl *SleepRateLimiter) TxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return srl.waitN(ctx, n, timeout)
}
