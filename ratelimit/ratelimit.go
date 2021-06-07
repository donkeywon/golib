package ratelimit

import (
	"github.com/donkeywon/golib/runner"
)

type RateLimiterType string

type RxTxRateLimiter interface {
	runner.Runner
	RxWaitN(n int, timeout int) error
	TxWaitN(n int, timeout int) error
}

type RateLimiterCfg struct {
	Cfg  interface{}     `json:"cfg"  yaml:"cfg"`
	Type RateLimiterType `json:"type" yaml:"type"`
}
