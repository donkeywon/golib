package ratelimit

import (
	"context"
	"fmt"
	"sync/atomic"
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
	plugin.RegCfg(RateLimiterTypeHost, func() any { return NewHostRateLimiterCfg() })
	plugin.Reg(RateLimiterTypeHost, func() any { return NewHostRateLimiter() })
}

const RateLimiterTypeHost RateLimiterType = "host"

type HostRateLimiterCfg struct {
	Nic             string `json:"nic"             validate:"required"    yaml:"nic"`
	MonitorInterval int    `json:"monitorInterval" yaml:"monitorInterval"`
	ReservePercent  int    `json:"reservePercent"  yaml:"reservePercent"`
	MinMBps         int    `json:"minMBps"         yaml:"minMBps"`
	FixedMBps       int    `json:"fixedMBps"       yaml:"fixedMBps"`
}

func NewHostRateLimiterCfg() *HostRateLimiterCfg {
	return &HostRateLimiterCfg{}
}

type HostRateLimiter struct {
	runner.Runner
	*HostRateLimiterCfg

	minMBps float64

	nicSpeedMbps int
	nicSpeedMBps int

	rxRL *rate.Limiter
	txRL *rate.Limiter

	rxPass     uint64
	txPass     uint64
	lastRxPass uint64
	lastTxPass uint64

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

	h.rxRL = rate.NewLimiter(0, 0)
	h.txRL = rate.NewLimiter(0, 0)

	if h.FixedMBps > 0 {
		h.Info("use fixed limit", "limit", i2MBps(h.FixedMBps))
		h.setRxTxLimit(float64(h.FixedMBps), float64(h.FixedMBps))
	} else {
		h.Info("use nic", "nic", h.Nic)
		h.nicSpeedMbps, err = eth.GetNicSpeed(h.Nic)
		if err != nil {
			h.Error("get nic speed fail", err)
			h.Info("try get nic speed on cloud")

			cloudType := cloud.Which()
			if cloudType == cloud.TypeUnknown {
				return errs.Errorf("unknown cloud type")
			}

			h.Info("host on cloud", "type", cloudType)
			h.nicSpeedMbps, err = cloud.GetNicSpeed()
			if err != nil {
				return errs.Wrapf(err, "get cloud(%s) network nic speed fail", cloudType)
			}
		}

		if h.nicSpeedMbps <= 0 {
			return errs.Errorf("nic speed must gt 0")
		}

		h.nicSpeedMBps = h.nicSpeedMbps / 8
		h.minMBps = float64(h.MinMBps)

		h.Info("nic speed", "B", i2MBps(h.nicSpeedMBps), "b", fmt.Sprintf("%d Mb/s", h.nicSpeedMbps))

		go h.monitor()
	}

	return h.Runner.Init()
}

func (h *HostRateLimiter) RxWaitN(n int, timeout int) error {
	ctx := h.Ctx()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(h.Ctx(), time.Second*time.Duration(timeout))
		defer cancel()
	}
	err := h.rxRL.WaitN(ctx, n)
	if err == nil {
		atomic.AddUint64(&h.rxPass, uint64(n))
	}
	return err
}

func (h *HostRateLimiter) TxWaitN(n int, timeout int) error {
	ctx := h.Ctx()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(h.Ctx(), time.Second*time.Duration(timeout))
		defer cancel()
	}
	err := h.txRL.WaitN(ctx, n)
	if err == nil {
		atomic.AddUint64(&h.txPass, uint64(n))
	}
	return err
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
			h.monitorCurSpeed()

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

func (h *HostRateLimiter) monitorCurSpeed() {
	rxPass := atomic.LoadUint64(&h.rxPass)
	txPass := atomic.LoadUint64(&h.txPass)
	h.Info("speed",
		"rx_speed", f2MBPS(float64(rxPass-h.lastRxPass)/1024/1024/float64(h.MonitorInterval)),
		"tx_speed", f2MBPS(float64(txPass-h.lastTxPass)/1024/1024/float64(h.MonitorInterval)))
	h.lastRxPass = rxPass
	h.lastTxPass = txPass
}

func (h *HostRateLimiter) setRxTxLimit(rxL float64, txL float64) {
	h.rxRL.SetLimit(rate.Limit(rxL * 1024 * 1024))
	h.txRL.SetLimit(rate.Limit(txL * 1024 * 1024))
}

func (h *HostRateLimiter) handleNetDevStats(stat *eth.NetDevStats) {
	curNicRxBytes := stat.RxBytes
	curNicTxBytes := stat.TxBytes

	if h.lastNicRxBytes == 0 || h.lastNicTxBytes == 0 {
		h.setRxTxLimit(h.minMBps, h.minMBps)
		h.Info("last rx or tx is 0, use min limit",
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
	rxLimit := calcLimit(rxMBps, float64(h.nicSpeedMBps), float64(h.ReservePercent), float64(h.MinMBps))
	txLimit := calcLimit(txMBps, float64(h.nicSpeedMBps), float64(h.ReservePercent), float64(h.MinMBps))
	h.Info("limit",
		"rx_limit", f2MBPS(rxLimit), "tx_limit", f2MBPS(txLimit),
		"nic_rx_bytes", curNicRxBytes, "nic_tx_bytes", curNicTxBytes,
		"max", i2MBps(h.nicSpeedMBps), "reserve_percent", h.ReservePercent, "min", i2MBps(h.MinMBps))
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

func calcLimit(cur float64, max float64, reservePercent float64, min float64) float64 {
	limit := max - (cur + max*reservePercent/100)
	if limit <= min {
		return min
	}
	return limit
}
