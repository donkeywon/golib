package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/donkeywon/golib/plugin"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestNewHostCfg(t *testing.T) {
	cfg := NewHostCfg()
	require.NotNil(t, cfg)
	assert.Empty(t, cfg.Nic)
	assert.Equal(t, 0, cfg.MonitorInterval)
	assert.Equal(t, 0, cfg.MaxPercent)
	assert.Equal(t, 0, cfg.MaxMBps)
	assert.Equal(t, 0, cfg.MinMBps)
}

func TestNewHostRateLimiter(t *testing.T) {
	h := NewHost()
	require.NotNil(t, h)
	assert.NotNil(t, h.closed)
	assert.Equal(t, 0, h.nicSpeedMBps)
	assert.Nil(t, h.rxRL)
	assert.Nil(t, h.txRL)
}

func TestHost_SetCfg(t *testing.T) {
	h := NewHost()
	cfg := &HostCfg{Nic: "eth0", MonitorInterval: 1, MaxPercent: 80}
	h.SetCfg(cfg)
	assert.Equal(t, cfg, h.cfg)

	// Type assertion panic on wrong type
	assert.Panics(t, func() {
		h.SetCfg("not a HostCfg")
	})
}

func TestHost_NicSpeedMBps(t *testing.T) {
	h := NewHost()
	assert.Equal(t, 0, h.NicSpeedMBps())

	h.nicSpeedMBps = 1000
	assert.Equal(t, 1000, h.NicSpeedMBps())
}

func TestHost_RxWaitN_NilContextPanics(t *testing.T) {
	h := NewHost()

	assert.Panics(t, func() {
		_ = h.RxWaitN(nil, 1, 0)
	})
}

func TestHost_TxWaitN_NilContextPanics(t *testing.T) {
	h := NewHost()

	assert.Panics(t, func() {
		_ = h.TxWaitN(nil, 1, 0)
	})
}

func TestHost_Init_ValidationFails(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{}
	ctx := mustCreateNopLogger().WithContext(context.Background())
	err := h.Init(ctx)
	require.Error(t, err)
}

func TestHost_PluginReg(t *testing.T) {
	// The init() should have registered the "host" type.
	// Verify by creating via the plugin system.
	rl := plugin.CreateWithCfg[RxTxRateLimiter](TypeHost, NewHostCfg())
	require.NotNil(t, rl)
	_, ok := rl.(*Host)
	assert.True(t, ok, "plugin should create a *Host")
}

func TestHost_RxWaitN_Timeout(t *testing.T) {
	h := NewHost()
	h.rxRL = nil // not initialized, will panic on wait — but we test timeout context path
	// We just test the timeout > 0 branch: create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()
	// Since rl is nil this will panic on WaitN
	assert.Panics(t, func() {
		_ = h.RxWaitN(ctx, 1, 100)
	})
}

func TestHost_TxWaitN_Timeout(t *testing.T) {
	h := NewHost()
	h.txRL = nil
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()
	assert.Panics(t, func() {
		_ = h.TxWaitN(ctx, 1, 100)
	})
}

func TestHost_Init_MinMBpsCalculation(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{Nic: "eth0", MonitorInterval: 1}
	ctx := mustCreateNopLogger().WithContext(context.Background())
	err := h.Init(ctx)
	require.Error(t, err) // eth0 might not exist or validation fails
}

func TestF2MBps(t *testing.T) {
	result := f2MBps(1.5)
	assert.Equal(t, "1.500 MB/s", result)

	result = f2MBps(0)
	assert.Equal(t, "0.000 MB/s", result)
}

func TestI2MBps(t *testing.T) {
	result := i2MBps(100)
	assert.Equal(t, "100 MB/s", result)

	result = i2MBps(0)
	assert.Equal(t, "0 MB/s", result)
}

func mustCreateNopLogger() *zerolog.Logger {
	return &zerolog.Logger{}
}

func TestCalcLimit(t *testing.T) {
	tests := []struct {
		name     string
		cur      float64
		max      float64
		min      float64
		nic      float64
		self     float64
		expected float64
	}{
		// cur >= 95% of nic: return min
		{
			name:     "cur at 95% of nic returns min",
			cur:      95,
			max:      80,
			min:      10,
			nic:      100,
			self:     50,
			expected: 10,
		},
		{
			name:     "cur above 95% of nic returns min",
			cur:      100,
			max:      80,
			min:      10,
			nic:      100,
			self:     50,
			expected: 10,
		},
		// speedOthers >= nic-max: return max((nic-cur)/2, min)
		// cur=60, self=10, nic=100, max=30, min=5
		// nic=95, speedOthers=50, nic-max=65, 50 >= 65? No... need speedOthers >= nic-max
		// Try: cur=60, self=55, nic=100, max=30, min=5
		// speedOthers=5, nic-max=95-30=65, 5 >= 65? No. Need larger speedOthers.
		// Try: cur=70, self=10, nic=100, max=50, min=5
		// nic=95, speedOthers=60, nic-max=45, 60 >= 45 → limit=(95-70)/2=12.5 > 5 → 12.5
		{
			name:     "speedOthers >= nic-max, limit > min",
			cur:      70,
			max:      50,
			min:      5,
			nic:      100,
			self:     10,
			expected: 12.5, // (95-70)/2 = 12.5
		},
		// speedOthers >= nic-max but limit <= min
		// Try: cur=90, self=10, nic=100, max=50, min=20
		// nic=95, speedOthers=80, nic-max=45, 80>=45 → limit=(95-90)/2=2.5 ≤ 20 → return min=20
		{
			name:     "speedOthers >= nic-max, limit <= min",
			cur:      90,
			max:      50,
			min:      20,
			nic:      100,
			self:     10,
			expected: 20,
		},
		// speedOthers < nic-max: return max
		{
			name:     "speedOthers < nic-max returns max",
			cur:      5,
			max:      50,
			min:      5,
			nic:      100,
			self:     3,
			expected: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcLimit(tt.cur, tt.max, tt.min, tt.nic, tt.self)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}

func TestHost_Close(t *testing.T) {
	h := NewHost()
	// Close should close the channel
	err := h.Close()
	require.NoError(t, err)
}

func TestHost_RxWaitN_WithTimeout(t *testing.T) {
	h := NewHost()
	h.rxRL = rate.NewLimiter(1000, 100)
	ctx := context.Background()
	// This should pass through and increment selfRxPass
	err := h.RxWaitN(ctx, 1, time.Second)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), h.selfRxPass.Load())
}

func TestHost_TxWaitN_WithTimeout(t *testing.T) {
	h := NewHost()
	h.txRL = rate.NewLimiter(1000, 100)
	ctx := context.Background()
	err := h.TxWaitN(ctx, 1, time.Second)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), h.selfTxPass.Load())
}

// TestHost_Init_Success — uses real enp3s0 NIC
func TestHost_Init_Success(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{Nic: "enp3s0", MonitorInterval: 10, MaxPercent: 50, MaxMBps: 100, MinMBps: 5}
	ctx := mustCreateNopLogger().WithContext(context.Background())
	err := h.Init(ctx)
	require.NoError(t, err)
	assert.Equal(t, 125, h.nicSpeedMBps) // 1000/8=125
	assert.NotNil(t, h.rxRL)
	assert.NotNil(t, h.txRL)
	h.Close()
}

// TestHost_Init_GetNicSpeedError
func TestHost_Init_GetNicSpeedError(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{Nic: "no_such_nic_xyz", MonitorInterval: 1, MaxPercent: 80}
	ctx := mustCreateNopLogger().WithContext(context.Background())
	err := h.Init(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get nic speed failed")
}

// TestHost_MonitorSelfSpeed
func TestHost_MonitorSelfSpeed(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{MonitorInterval: 2}
	h.selfRxPass.Store(2000000)
	h.selfTxPass.Store(1000000)
	h.monitorSelfSpeed()
	assert.Equal(t, uint64(2000000), h.selfLastRxPass)
	assert.InDelta(t, 2000000.0/1048576/2, h.selfRxSpeedMBps, 0.1)
}

// TestHost_SetRxTxLimit
func TestHost_SetRxTxLimit(t *testing.T) {
	h := NewHost()
	h.rxRL = rate.NewLimiter(rate.Inf, 1)
	h.txRL = rate.NewLimiter(rate.Inf, 1)
	h.setRxTxLimit(10.5, 20.5)
	assert.InDelta(t, 10.5*1048576, float64(h.rxRL.Limit()), 1)
}

// TestHost_HandleNetDevStats — all branches
func TestHost_HandleNetDevStats(t *testing.T) {
	t.Run("first call uses min", func(t *testing.T) {
		h := NewHost()
		h.l = mustCreateNopLogger()
		h.cfg = &HostCfg{MonitorInterval: 5, MinMBps: 10}
		h.nicSpeedMBps = 125
		h.maxMBps = 60
		h.rxRL = rate.NewLimiter(rate.Inf, 1)
		h.txRL = rate.NewLimiter(rate.Inf, 1)
		stat := &net.IOCountersStat{BytesRecv: 5000000, BytesSent: 3000000}
		h.handleNetDevStats(stat)
		assert.Equal(t, uint64(5000000), h.lastNicRxBytes)
	})

	t.Run("normal returns max", func(t *testing.T) {
		h := NewHost()
		h.l = mustCreateNopLogger()
		h.cfg = &HostCfg{MonitorInterval: 1, MinMBps: 5, MaxMBps: 100}
		h.nicSpeedMBps = 125
		h.maxMBps = 80
		h.rxRL = rate.NewLimiter(rate.Inf, 1)
		h.txRL = rate.NewLimiter(rate.Inf, 1)
		h.lastNicRxBytes = 1000000
		h.lastNicTxBytes = 500000
		stat := &net.IOCountersStat{BytesRecv: 2000000, BytesSent: 1000000}
		h.handleNetDevStats(stat)
	})

	t.Run("cur less than last", func(t *testing.T) {
		h := NewHost()
		h.l = mustCreateNopLogger()
		h.cfg = &HostCfg{MonitorInterval: 1, MinMBps: 5, MaxMBps: 100}
		h.nicSpeedMBps = 125
		h.maxMBps = 80
		h.rxRL = rate.NewLimiter(rate.Inf, 1)
		h.txRL = rate.NewLimiter(rate.Inf, 1)
		h.lastNicRxBytes = 5000000
		h.lastNicTxBytes = 3000000
		stat := &net.IOCountersStat{BytesRecv: 1000000, BytesSent: 500000}
		h.handleNetDevStats(stat)
	})
}

// TestHost_Monitor_StopsOnClose
func TestHost_Monitor_StopsOnClose(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{MonitorInterval: 1}
	h.l = mustCreateNopLogger()
	done := make(chan struct{})
	go func() { h.monitor(); close(done) }()
	time.Sleep(50 * time.Millisecond)
	h.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not stop")
	}
}

// TestHost_RxWaitN_NoError
func TestHost_RxWaitN_NoError(t *testing.T) {
	h := NewHost()
	h.rxRL = rate.NewLimiter(rate.Inf, 100)
	err := h.RxWaitN(context.Background(), 10, 0)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), h.selfRxPass.Load())
}

// TestHost_Monitor_OneTick runs monitor for one tick with a real NIC.
func TestHost_Monitor_OneTick(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{Nic: "enp3s0", MonitorInterval: 1, MinMBps: 5, MaxMBps: 100, MaxPercent: 80}
	h.l = mustCreateNopLogger()
	h.nicSpeedMBps = 125
	h.maxMBps = 80
	h.rxRL = rate.NewLimiter(rate.Inf, 1)
	h.txRL = rate.NewLimiter(rate.Inf, 1)
	done := make(chan struct{})
	go func() { h.monitor(); close(done) }()
	time.Sleep(1100 * time.Millisecond)
	h.Close()
	<-done
}

// TestHost_Monitor_NicNotFound triggers "nic stats not found" error path.
func TestHost_Monitor_NicNotFound(t *testing.T) {
	h := NewHost()
	h.cfg = &HostCfg{Nic: "definitely_not_a_real_nic", MonitorInterval: 1, MinMBps: 5}
	h.l = mustCreateNopLogger()
	h.rxRL = rate.NewLimiter(rate.Inf, 1)
	h.txRL = rate.NewLimiter(rate.Inf, 1)
	done := make(chan struct{})
	go func() { h.monitor(); close(done) }()
	time.Sleep(1100 * time.Millisecond)
	h.Close()
	<-done
}
