package ratelimit

import (
	"context"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"golang.org/x/time/rate"
)

func init() {
	plugin.RegWithCfg(TypeFixed, func() RxTxRateLimiter { return NewFixedRateLimiter() }, func() any { return NewFixedRateLimiterCfg() })
}

const TypeFixed Type = "fixed"

type FixedRateLimiterCfg struct {
	N     int
	Burst int
}

func NewFixedRateLimiterCfg() *FixedRateLimiterCfg {
	return &FixedRateLimiterCfg{}
}

type FixedRateLimiter struct {
	runner.Runner
	*FixedRateLimiterCfg
	txRl *rate.Limiter
	rxRl *rate.Limiter
}

func NewFixedRateLimiter() *FixedRateLimiter {
	return &FixedRateLimiter{
		Runner:              runner.Create("fixedRateLimiter"),
		FixedRateLimiterCfg: NewFixedRateLimiterCfg(),
	}
}

func (frl *FixedRateLimiter) Init() error {
	if frl.N < 0 {
		return errs.Errorf("fixed rate limiter N must ge 0: %d", frl.N)
	}
	if frl.Burst <= 0 {
		return errs.Errorf("fixed rate limiter Burst must gt 0: %d", frl.Burst)
	}
	frl.rxRl = rate.NewLimiter(rate.Limit(frl.N), frl.Burst)
	frl.txRl = rate.NewLimiter(rate.Limit(frl.N), frl.Burst)
	return frl.Runner.Init()
}

func (frl *FixedRateLimiter) waitN(ctx context.Context, n int, timeout time.Duration, rl *rate.Limiter) error {
	if n == 0 {
		return nil
	}

	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return rl.WaitN(ctx, n)
}

func (frl *FixedRateLimiter) RxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return frl.waitN(ctx, n, timeout, frl.rxRl)
}

func (frl *FixedRateLimiter) TxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return frl.waitN(ctx, n, timeout, frl.txRl)
}

func (frl *FixedRateLimiter) SetRxLimit(n int, burst int) {
	frl.rxRl.SetLimit(rate.Limit(n))
	frl.rxRl.SetBurst(burst)
}

func (frl *FixedRateLimiter) SetTxLimit(n int, burst int) {
	frl.txRl.SetLimit(rate.Limit(n))
	frl.txRl.SetBurst(burst)
}
