package ratelimit

import (
	"fmt"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestHost(t *testing.T) {
	hc := NewHostRateLimiterCfg()
	hc.Nic = "eth3"
	hc.MinMBps = 10
	hc.MonitorInterval = 1
	hc.MaxPercent = 10

	h := NewHostRateLimiter()
	h.HostRateLimiterCfg = hc

	tests.DebugInit(h)

	require.NoError(t, h.Init())

	time.Sleep(time.Second * 1000)
}

func TestCalcLimit(t *testing.T) {
	nic := 3500.0
	max := 1000.0
	min := 10.0
	self := 100.0

	for other := 100.0; other < 3500; other += 100 {
		self = calcLimit(other+self, max, min, nic, self)

		fmt.Println(other, self)
	}
}
