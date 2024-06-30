package ratelimit

import (
	"testing"
	"time"

	"github.com/donkeywon/golib/util/test"
	"github.com/stretchr/testify/require"
)

func TestHost(t *testing.T) {
	hc := NewHostRateLimiterCfg()
	hc.Nic = "eth3"
	hc.FixedMBps = 0
	hc.MinMBps = 10
	hc.MonitorInterval = 1
	hc.ReservePercent = 10

	h := NewHostRateLimiter()
	h.HostRateLimiterCfg = hc

	test.DebugInherit(h)

	require.NoError(t, h.Init())

	time.Sleep(time.Second * 1000)
}
