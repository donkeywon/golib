package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cloud"
	"github.com/donkeywon/golib/util/eth"
	"github.com/donkeywon/golib/util/v"
	"golang.org/x/time/rate"
)

func init() {
	plugin.RegWithCfg(TypeHost, func() RxTxRateLimiter { return NewHostRateLimiter() }, func() any { return NewHostRateLimiterCfg() })
}

const TypeHost Type = "host"

type HostRateLimiterCfg struct {
	Nic              string `json:"nic"              yaml:"nic" validate:"required"`
	MonitorInterval  int    `json:"monitorInterval"  yaml:"monitorInterval"`
	ReservePercent   int    `json:"reservePercent"   yaml:"reservePercent"`
	ReserveFixedMBps int    `json:"reserveFixedMBps" yaml:"reserveFixedMBps"`
	MinMBps          int    `json:"minMBps"          yaml:"minMBps"`
}

func NewHostRateLimiterCfg() *HostRateLimiterCfg {
	return &HostRateLimiterCfg{}
}

type HostRateLimiter struct {
	runner.Runner
	*HostRateLimiterCfg

	reserveMBps int
	minMBps     float64

	nicSpeedMbps int
	nicSpeedMBps int

	rxRL *rate.Limiter
	txRL *rate.Limiter

	lastNicRxBytes uint64
	lastNicTxBytes uint64
}

func NewHostRateLimiter() *HostRateLimiter {
	return &HostRateLimiter{
		Runner:             runner.Create("hostRateLimiter"),
		HostRateLimiterCfg: NewHostRateLimiterCfg(),
	}
}

func (h *HostRateLimiter) Init() error {
	err := v.Struct(h.HostRateLimiterCfg)
	if err != nil {
		return err
	}

	h.Info("use nic", "nic", h.Nic)
	h.nicSpeedMbps, err = eth.GetNicSpeed(h.Nic)
	if err != nil {
		h.Error("get nic speed failed", err)
		h.Info("try get nic speed on cloud")

		cloudType := cloud.Which()
		if cloudType == cloud.TypeUnknown {
			return errs.Errorf("unknown cloud type")
		}

		h.Info("host on cloud", "type", cloudType)
		h.nicSpeedMbps, err = cloud.GetNicSpeed()
		if err != nil {
			return errs.Wrapf(err, "get cloud(%s) network nic speed failed", cloudType)
		}
	}

	if h.nicSpeedMbps <= 0 {
		return errs.Errorf("nic speed must gt 0")
	}

	h.nicSpeedMBps = h.nicSpeedMbps / 8
	h.reserveMBps = h.nicSpeedMBps / h.ReservePercent
	if h.ReserveFixedMBps > h.reserveMBps {
		h.reserveMBps = h.ReserveFixedMBps
	}
	h.minMBps = float64(h.MinMBps)

	initRateLimitBurst := h.nicSpeedMBps - h.reserveMBps
	h.rxRL = rate.NewLimiter(rate.Limit(initRateLimitBurst*1048576), initRateLimitBurst*1048576)
	h.txRL = rate.NewLimiter(rate.Limit(initRateLimitBurst*1048576), initRateLimitBurst*1048576)

	h.Info("nic rate limit info",
		"nic_speed", i2MBps(h.nicSpeedMBps),
		"reserve", i2MBps(h.reserveMBps),
		"min", i2MBps(h.MinMBps),
		"burst", i2MBps(initRateLimitBurst),
	)

	go h.monitor()

	return h.Runner.Init()
}

func (h *HostRateLimiter) RxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	if ctx == nil {
		ctx = h.Ctx()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(h.Ctx(), timeout)
		defer cancel()
	}
	return h.rxRL.WaitN(ctx, n)
}

func (h *HostRateLimiter) TxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	if ctx == nil {
		ctx = h.Ctx()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(h.Ctx(), timeout)
		defer cancel()
	}
	return h.txRL.WaitN(ctx, n)
}

func (h *HostRateLimiter) monitor() {
	interval := time.Duration(h.MonitorInterval) * time.Second
	t := time.NewTicker(interval)
	defer t.Stop()
	nic := h.Nic
	for {
		select {
		case <-h.Stopping():
			return
		case <-t.C:
			stats, err := eth.GetNetDevStats()
			if err != nil {
				h.setRxTxLimit(h.minMBps, h.minMBps)
				h.Error("get net dev stats fail, use min limit", err, "min_limit", i2MBps(h.MinMBps))
				continue
			}

			if _, exists := stats[nic]; !exists {
				h.setRxTxLimit(h.minMBps, h.minMBps)
				h.Error("nic stats is empty, use min limit", nil, "min_limit", i2MBps(h.MinMBps), "stats", stats)
				continue
			}

			h.handleNetDevStats(stats[nic])
		}
	}
}

func (h *HostRateLimiter) setRxTxLimit(rxL float64, txL float64) {
	h.rxRL.SetLimit(rate.Limit(rxL * 1048576))
	h.txRL.SetLimit(rate.Limit(txL * 1048576))
}

func (h *HostRateLimiter) handleNetDevStats(stat *eth.NetDevStats) {
	curNicRxBytes := stat.RxBytes
	curNicTxBytes := stat.TxBytes

	if h.lastNicRxBytes == 0 || h.lastNicTxBytes == 0 {
		h.setRxTxLimit(h.minMBps, h.minMBps)
		h.Debug("last rx or tx is 0, use min limit",
			"last_nic_rx_bytes", h.lastNicRxBytes, "last_nic_tx_bytes", h.lastNicTxBytes, "min", i2MBps(h.MinMBps))
		h.lastNicRxBytes = curNicRxBytes
		h.lastNicTxBytes = curNicTxBytes
		return
	}

	var (
		rxSub uint64
		txSub uint64
	)
	if curNicRxBytes < h.lastNicRxBytes {
		h.Warn("cur rx bytes is small than last rx bytes",
			"last_nic_rx_bytes", h.lastNicRxBytes, "cur_nic_rx_bytes", curNicRxBytes)
	} else {
		rxSub = curNicRxBytes - h.lastNicRxBytes
	}
	if curNicTxBytes < h.lastNicTxBytes {
		h.Warn("cur tx bytes is small than last tx bytes",
			"last_nic_rx_bytes", h.lastNicTxBytes, "cur_nic_rx_bytes", curNicTxBytes)
	} else {
		txSub = curNicTxBytes - h.lastNicTxBytes
	}

	rxMBps := float64(rxSub) / 1024 / 1024 / float64(h.MonitorInterval)
	txMBps := float64(txSub) / 1024 / 1024 / float64(h.MonitorInterval)
	rxLimit := calcLimit(rxMBps, float64(h.nicSpeedMBps), float64(h.reserveMBps), h.minMBps)
	txLimit := calcLimit(txMBps, float64(h.nicSpeedMBps), float64(h.reserveMBps), h.minMBps)
	h.Info("nic limit",
		"rx_limit", f2MBPS(rxLimit), "tx_limit", f2MBPS(txLimit),
		"nic_rx_bytes", curNicRxBytes, "nic_tx_bytes", curNicTxBytes,
		"max", i2MBps(h.nicSpeedMBps), "reserve", i2MBps(h.reserveMBps), "min", i2MBps(h.MinMBps))
	h.setRxTxLimit(rxLimit, txLimit)

	h.lastNicRxBytes = curNicRxBytes
	h.lastNicTxBytes = curNicTxBytes
}

func f2MBPS(f float64) string {
	return fmt.Sprintf("%.3f MB/s", f)
}

func i2MBps(i int) string {
	return fmt.Sprintf("%d MB/s", i)
}

func calcLimit(cur float64, max float64, reserve float64, min float64) float64 {
	limit := max - (cur + reserve)
	if limit <= min {
		return min
	}
	return limit
}
