package ratelimit

import (
	"context"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"golang.org/x/time/rate"
)

func init() {
	plugin.Reg(TypeFixed, func() RxTxRateLimiter { return NewFixedRateLimiter() }, func() any { return NewFixedRateLimiterCfg() })
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
	cfg  *FixedRateLimiterCfg
	txRl *rate.Limiter
	rxRl *rate.Limiter
}

func NewFixedRateLimiter() *FixedRateLimiter {
	return &FixedRateLimiter{}
}

func (frl *FixedRateLimiter) SetCfg(cfg any) {
	frl.cfg = cfg.(*FixedRateLimiterCfg)
}

func (frl *FixedRateLimiter) Init(ctx context.Context) error {
	if frl.cfg.N < 0 {
		return errs.Errorf("fixed rate limiter N must ge 0: %d", frl.cfg.N)
	}
	if frl.cfg.Burst <= 0 {
		return errs.Errorf("fixed rate limiter Burst must gt 0: %d", frl.cfg.Burst)
	}
	frl.rxRl = rate.NewLimiter(rate.Limit(frl.cfg.N), frl.cfg.Burst)
	frl.txRl = rate.NewLimiter(rate.Limit(frl.cfg.N), frl.cfg.Burst)
	return nil
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
