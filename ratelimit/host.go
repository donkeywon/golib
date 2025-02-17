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
	plugin.RegWithCfg(TypeHost, func() RxTxRateLimiter { return NewHostRateLimiter() }, func() any { return NewHostRateLimiterCfg() })
}

const TypeHost Type = "host"

type HostRateLimiterCfg struct {
	Nic             string `json:"nic"              yaml:"nic"               validate:"required"`
	MonitorInterval int    `json:"monitorInterval"  yaml:"monitorInterval"   validate:"required"`
	MaxPercent      int    `json:"maxPercent"       yaml:"maxPercent"        validate:"gte=0,lte=100"`
	MaxMBps         int    `json:"maxMBps"          yaml:"maxMBps"`
	MinMBps         int    `json:"minMBps"          yaml:"minMBps"           validate:"gte=0"`
}

func NewHostRateLimiterCfg() *HostRateLimiterCfg {
	return &HostRateLimiterCfg{}
}

type HostRateLimiter struct {
	runner.Runner
	*HostRateLimiterCfg

	nicSpeedMBps int
	maxMBps      int

	rxRL *rate.Limiter
	txRL *rate.Limiter

	selfRxPass      uint64
	selfTxPass      uint64
	selfLastRxPass  uint64
	selfLastTxPass  uint64
	selfRxSpeedMBps float64
	selfTxSpeedMBps float64

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
	nicSpeedMbps, err := eth.GetNicSpeed(h.Nic)
	if err != nil {
		h.Error("get nic speed failed", err)
		h.Info("try get nic speed on cloud")

		cloudType := cloud.Which()
		if cloudType == cloud.TypeUnknown {
			return errs.Errorf("unknown cloud type")
		}

		h.Info("host on cloud", "type", cloudType)
		nicSpeedMbps, err = cloud.GetNicSpeed()
		if err != nil {
			return errs.Wrapf(err, "get cloud(%s) network nic speed failed", cloudType)
		}
	}

	if nicSpeedMbps <= 0 {
		return errs.Errorf("nic speed must gt 0")
	}

	h.nicSpeedMBps = nicSpeedMbps / 8
	h.maxMBps = h.nicSpeedMBps * h.MaxPercent / 100
	if h.MaxMBps < h.maxMBps {
		h.maxMBps = h.MaxMBps
	}

	h.rxRL = rate.NewLimiter(rate.Limit(h.maxMBps*1048576), h.maxMBps*1048576)
	h.txRL = rate.NewLimiter(rate.Limit(h.maxMBps*1048576), h.maxMBps*1048576)

	h.Info("nic rate limit info",
		"nic_speed", i2MBps(h.nicSpeedMBps),
		"max", i2MBps(h.maxMBps),
		"min", i2MBps(h.MinMBps),
	)

	go h.monitor()

	return h.Runner.Init()
}

func (h *HostRateLimiter) NicSpeedMBps() int {
	return h.nicSpeedMBps
}

func (h *HostRateLimiter) RxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	if ctx == nil {
		panic("nil context")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(h.Ctx(), timeout)
		defer cancel()
	}
	err := h.rxRL.WaitN(ctx, n)
	if err == nil {
		atomic.AddUint64(&h.selfRxPass, uint64(n))
	}
	return err
}

func (h *HostRateLimiter) TxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	if ctx == nil {
		panic("nil context")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(h.Ctx(), timeout)
		defer cancel()
	}
	err := h.txRL.WaitN(ctx, n)
	if err == nil {
		atomic.AddUint64(&h.selfTxPass, uint64(n))
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
			h.monitorSelfSpeed()

			stats, err := eth.GetNetDevStats()
			if err != nil {
				h.setRxTxLimit(float64(h.MinMBps), float64(h.MinMBps))
				h.Error("get net dev stats fail, use min limit", err, "min_limit", i2MBps(h.MinMBps))
				continue
			}

			if _, exists := stats[nic]; !exists {
				h.setRxTxLimit(float64(h.MinMBps), float64(h.MinMBps))
				h.Error("nic stats is empty, use min limit", nil, "min_limit", i2MBps(h.MinMBps), "stats", stats)
				continue
			}

			h.handleNetDevStats(stats[nic])
		}
	}
}

func (h *HostRateLimiter) monitorSelfSpeed() {
	rxPass := atomic.LoadUint64(&h.selfRxPass)
	txPass := atomic.LoadUint64(&h.selfTxPass)
	h.selfRxSpeedMBps = float64(rxPass-h.selfLastRxPass) / 1048576 / float64(h.MonitorInterval)
	h.selfTxSpeedMBps = float64(txPass-h.selfLastTxPass) / 1048576 / float64(h.MonitorInterval)
	h.selfLastRxPass = rxPass
	h.selfLastTxPass = txPass
}

func (h *HostRateLimiter) setRxTxLimit(rxL float64, txL float64) {
	h.rxRL.SetLimit(rate.Limit(rxL * 1048576))
	h.txRL.SetLimit(rate.Limit(txL * 1048576))
}

func (h *HostRateLimiter) handleNetDevStats(stat *eth.NetDevStats) {
	curNicRxBytes := stat.RxBytes
	curNicTxBytes := stat.TxBytes

	if h.lastNicRxBytes == 0 || h.lastNicTxBytes == 0 {
		h.setRxTxLimit(float64(h.MinMBps), float64(h.MinMBps))
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

	rxMBps := float64(rxSub) / float64(1048576) / float64(h.MonitorInterval)
	txMBps := float64(txSub) / float64(1048576) / float64(h.MonitorInterval)
	rxLimit := calcLimit(rxMBps, float64(h.maxMBps), float64(h.MinMBps), float64(h.nicSpeedMBps), h.selfRxSpeedMBps)
	txLimit := calcLimit(txMBps, float64(h.maxMBps), float64(h.MinMBps), float64(h.nicSpeedMBps), h.selfTxSpeedMBps)
	h.Info("nic limit",
		"nic_speed", i2MBps(h.nicSpeedMBps), "rx_speed", f2MBps(rxMBps), "tx_speed", f2MBps(txMBps),
		"rx_limit", f2MBps(rxLimit), "tx_limit", f2MBps(txLimit),
		"max", i2MBps(h.maxMBps), "min", i2MBps(h.MinMBps),
		"nic_rx_bytes", curNicRxBytes, "nic_tx_bytes", curNicTxBytes,
	)
	h.setRxTxLimit(rxLimit, txLimit)

	h.lastNicRxBytes = curNicRxBytes
	h.lastNicTxBytes = curNicTxBytes
}

func f2MBps(f float64) string {
	return fmt.Sprintf("%.3f MB/s", f)
}

func i2MBps(i int) string {
	return fmt.Sprintf("%d MB/s", i)
}

func calcLimit(cur float64, max float64, min float64, nic float64, self float64) float64 {
	// cur = self + others

	// if cur is 95% of nic (in fact, cur is always close to nic but not equal, because cur is calculated),
	// we think nic bandwidth is fully used, so I use min
	nic = nic * 0.95
	if cur >= nic {
		return min
	}

	// nic bandwidth is not fully used
	// speedOthers is greater than nic - max; I use (nic-cur)/2,
	speedOthers := cur - self
	if speedOthers >= nic-max {
		limit := (nic - cur) / 2
		if limit <= min {
			limit = min
		}

		return limit
	}

	// speedOthers is smaller than nic - max，I can use max
	return max
}
