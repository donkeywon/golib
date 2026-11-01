package ratelimit

import (
	"context"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"golang.org/x/time/rate"
)

func init() {
	plugin.Reg(TypeFixed, func() RxTxRateLimiter { return NewFixed() }, NewFixedCfg)
}

const TypeFixed Type = "fixed"

type FixedCfg struct {
	N     int
	Burst int
}

func NewFixedCfg() *FixedCfg {
	return &FixedCfg{}
}

type Fixed struct {
	cfg  *FixedCfg
	txRl *rate.Limiter
	rxRl *rate.Limiter
}

func NewFixed() *Fixed {
	return &Fixed{}
}

func (frl *Fixed) SetCfg(cfg any) {
	frl.cfg = cfg.(*FixedCfg)
}

func (frl *Fixed) Init(ctx context.Context) error {
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

func (frl *Fixed) waitN(ctx context.Context, n int, timeout time.Duration, rl *rate.Limiter) error {
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

func (frl *Fixed) RxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return frl.waitN(ctx, n, timeout, frl.rxRl)
}

func (frl *Fixed) TxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	return frl.waitN(ctx, n, timeout, frl.txRl)
}

func (frl *Fixed) SetRxLimit(n int, burst int) {
	frl.rxRl.SetLimit(rate.Limit(n))
	frl.rxRl.SetBurst(burst)
}

func (frl *Fixed) SetTxLimit(n int, burst int) {
	frl.txRl.SetLimit(rate.Limit(n))
	frl.txRl.SetBurst(burst)
}
