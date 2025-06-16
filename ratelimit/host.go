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
				h.setRxTxLimit(h.MinMBps, h.MinMBps)
				h.Error("get net dev stats fail, use min limit", err, "min_limit", i2MBps(h.MinMBps))
				continue
			}

			if _, exists := stats[nic]; !exists {
				h.setRxTxLimit(h.MinMBps, h.MinMBps)
				h.Error("nic stats is empty, use min limit", nil, "min_limit", i2MBps(h.MinMBps), "stats", stats)
				continue
			}

			h.handleNetDevStats(stats[nic])
		}
	}
}

func (h *HostRateLimiter) setRxTxLimit(rxL int, txL int) {
	h.rxRL.SetLimit(rate.Limit(rxL * 1048576))
	h.txRL.SetLimit(rate.Limit(txL * 1048576))
}

func (h *HostRateLimiter) handleNetDevStats(stat *eth.NetDevStats) {
	curNicRxBytes := stat.RxBytes
	curNicTxBytes := stat.TxBytes

	if h.lastNicRxBytes == 0 || h.lastNicTxBytes == 0 {
		h.setRxTxLimit(h.MinMBps, h.MinMBps)
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

	rxMBps := int(rxSub / 1024 / 1024 / uint64(h.MonitorInterval))
	txMBps := int(txSub / 1024 / 1024 / uint64(h.MonitorInterval))
	rxLimit := calcLimit(rxMBps, h.maxMBps, h.MinMBps)
	txLimit := calcLimit(txMBps, h.maxMBps, h.MinMBps)
	h.Info("nic limit",
		"nic_speed", i2MBps(h.nicSpeedMBps), "rx_speed", i2MBps(rxMBps), "tx_speed", i2MBps(txMBps),
		"rx_limit", i2MBps(rxLimit), "tx_limit", i2MBps(txLimit),
		"max", i2MBps(h.maxMBps), "min", i2MBps(h.MinMBps),
		"nic_rx_bytes", curNicRxBytes, "nic_tx_bytes", curNicTxBytes,
	)
	h.setRxTxLimit(rxLimit, txLimit)

	h.lastNicRxBytes = curNicRxBytes
	h.lastNicTxBytes = curNicTxBytes
}

func i2MBps(i int) string {
	return fmt.Sprintf("%d MB/s", i)
}

func calcLimit(cur int, max int, min int) int {
	limit := max - cur
	if limit <= min {
		return min
	}
	return limit
}
