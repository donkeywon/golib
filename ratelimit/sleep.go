package ratelimit

import (
	"context"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.Reg(TypeSleep, func() RxTxRateLimiter { return NewSleepRateLimiter() }, func() any { return NewSleepRateLimiterCfg() })
}

const TypeSleep Type = "sleep"

type SleepRateLimiterCfg struct {
	Duration time.Duration `json:"duration" yaml:"duration" validate:"required"`
}

func NewSleepRateLimiterCfg() *SleepRateLimiterCfg {
	return &SleepRateLimiterCfg{}
}

type SleepRateLimiter struct {
	cfg *SleepRateLimiterCfg
}

func NewSleepRateLimiter() *SleepRateLimiter {
	return &SleepRateLimiter{}
}

func (srl *SleepRateLimiter) Init(ctx context.Context) error {
	if srl.cfg.Duration <= 0 {
		return errs.Errorf("sleep rate limiter duration must gt 0: %d", srl.cfg.Duration)
	}
	return nil
}

func (srl *SleepRateLimiter) waitN(ctx context.Context, n int, timeout time.Duration) error {
	if n == 0 {
		return nil
	}

	d := srl.cfg.Duration
	if timeout > 0 && timeout < d {
		d = timeout
	}

	select {
	case <-ctx.Done():
	case <-time.After(d):
	}

	return nil
}

func (srl *SleepRateLimiter) RxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return srl.waitN(ctx, n, timeout)
}

func (srl *SleepRateLimiter) TxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return srl.waitN(ctx, n, timeout)
}
