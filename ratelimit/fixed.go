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
	N int
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
	if frl.N <= 0 {
		return errs.Errorf("fixed rate limiter N must gt 0: %d", frl.N)
	}
	frl.rxRl = rate.NewLimiter(rate.Limit(frl.N), frl.N)
	frl.txRl = rate.NewLimiter(rate.Limit(frl.N), frl.N)
	return frl.Runner.Init()
}

func (frl *FixedRateLimiter) waitN(n int, timeout int, rl *rate.Limiter) error {
	if n == 0 {
		return nil
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Millisecond*time.Duration(timeout))
		defer cancel()
	}
	return rl.WaitN(ctx, n)
}

func (frl *FixedRateLimiter) RxWaitN(n int, timeout int) error {
	return frl.waitN(n, timeout, frl.rxRl)
}

func (frl *FixedRateLimiter) TxWaitN(n int, timeout int) error {
	return frl.waitN(n, timeout, frl.txRl)
}

func (frl *FixedRateLimiter) Type() Type {
	return TypeFixed
}
