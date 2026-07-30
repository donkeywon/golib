package ratelimit

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/eth"
	"github.com/donkeywon/golib/util/v"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v4/net"
	"golang.org/x/time/rate"
)

func init() {
	plugin.Reg(TypeHost, func() RxTxRateLimiter { return NewHost() }, func() any { return NewHostCfg() })
}

const TypeHost Type = "host"

type HostCfg struct {
	Nic             string `json:"nic"              yaml:"nic"               validate:"required"`
	MonitorInterval int    `json:"monitor_interval" yaml:"monitorInterval"   validate:"required"`
	MaxPercent      int    `json:"max_percent"      yaml:"maxPercent"        validate:"gte=0,lte=100"`
	MaxMBps         int    `json:"max_mbps"         yaml:"maxMBps"           validate:"gte=0"`
	MinMBps         int    `json:"min_mbps"         yaml:"minMBps"           validate:"gte=0"`
}

func NewHostCfg() *HostCfg {
	return &HostCfg{}
}

type Host struct {
	cfg *HostCfg

	nicSpeedMBps int
	maxMBps      int

	rxRL *rate.Limiter
	txRL *rate.Limiter

	selfRxPass      atomic.Uint64
	selfTxPass      atomic.Uint64
	selfLastRxPass  uint64
	selfLastTxPass  uint64
	selfRxSpeedMBps float64
	selfTxSpeedMBps float64

	lastNicRxBytes uint64
	lastNicTxBytes uint64

	l      *zerolog.Logger
	closed chan struct{}
}

func NewHost() *Host {
	return &Host{
		closed: make(chan struct{}),
	}
}

func (h *Host) SetCfg(cfg any) {
	h.cfg = cfg.(*HostCfg)
}

func (h *Host) Init(ctx context.Context) error {
	err := v.Struct(h.cfg)
	if err != nil {
		return err
	}

	h.l = zerolog.Ctx(ctx)

	h.l.Info().Str("nic", h.cfg.Nic).Msg("use nic")
	nicSpeedMbps, err := eth.GetNicSpeed(ctx, h.cfg.Nic)
	if err != nil {
		return errs.Wrapf(err, "get nic speed failed: %s", h.cfg.Nic)
	}

	if nicSpeedMbps <= 0 {
		return errs.Errorf("nic speed must gt 0")
	}

	h.nicSpeedMBps = nicSpeedMbps / 8
	h.maxMBps = min(h.nicSpeedMBps*h.cfg.MaxPercent/100, h.cfg.MaxMBps)

	h.rxRL = rate.NewLimiter(rate.Limit(h.maxMBps*1048576), h.maxMBps*1048576)
	h.txRL = rate.NewLimiter(rate.Limit(h.maxMBps*1048576), h.maxMBps*1048576)

	h.l.Info().Str("nic_speed", i2MBps(h.nicSpeedMBps)).Str("max", i2MBps(h.maxMBps)).Str("min", i2MBps(h.cfg.MinMBps)).Msg("nic rate limit info")

	go h.monitor()

	return nil
}

func (h *Host) Close() error {
	close(h.closed)
	return nil
}

func (h *Host) NicSpeedMBps() int {
	return h.nicSpeedMBps
}

func (h *Host) RxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	if ctx == nil {
		panic("nil context")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	err := h.rxRL.WaitN(ctx, n)
	if err == nil {
		h.selfRxPass.Add(uint64(n))
	}
	return err
}

func (h *Host) TxWaitN(ctx context.Context, n int, timeout time.Duration) error {
	if ctx == nil {
		panic("nil context")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	err := h.txRL.WaitN(ctx, n)
	if err == nil {
		h.selfTxPass.Add(uint64(n))
	}
	return err
}

func (h *Host) monitor() {
	interval := time.Duration(h.cfg.MonitorInterval) * time.Second
	t := time.NewTicker(interval)
	defer t.Stop()
	nic := h.cfg.Nic
	for {
		select {
		case <-h.closed:
			return
		case <-t.C:
			h.monitorSelfSpeed()

			stats, err := net.IOCounters(true)
			if err != nil {
				h.setRxTxLimit(float64(h.cfg.MinMBps), float64(h.cfg.MinMBps))
				h.l.Error().Err(err).Str("min_limit", i2MBps(h.cfg.MinMBps)).Msg("get nic stats fail, use min limit")
				continue
			}

			var i int
			for i = range stats {
				if stats[i].Name == nic {
					break
				}
			}
			if i == len(stats) {
				h.setRxTxLimit(float64(h.cfg.MinMBps), float64(h.cfg.MinMBps))
				h.l.Error().Str("nic", nic).Str("min_limit", i2MBps(h.cfg.MinMBps)).Any("stats", stats).Msg("nic stats not found, use min limit")
				continue
			}

			h.handleNetDevStats(&stats[i])
		}
	}
}

func (h *Host) monitorSelfSpeed() {
	rxPass := h.selfRxPass.Load()
	txPass := h.selfTxPass.Load()
	h.selfRxSpeedMBps = float64(rxPass-h.selfLastRxPass) / 1048576 / float64(h.cfg.MonitorInterval)
	h.selfTxSpeedMBps = float64(txPass-h.selfLastTxPass) / 1048576 / float64(h.cfg.MonitorInterval)
	h.selfLastRxPass = rxPass
	h.selfLastTxPass = txPass
}

func (h *Host) setRxTxLimit(rxL float64, txL float64) {
	h.rxRL.SetLimit(rate.Limit(rxL * 1048576))
	h.txRL.SetLimit(rate.Limit(txL * 1048576))
}

func (h *Host) handleNetDevStats(stat *net.IOCountersStat) {
	curNicRxBytes := stat.BytesRecv
	curNicTxBytes := stat.BytesSent

	if h.lastNicRxBytes == 0 || h.lastNicTxBytes == 0 {
		h.setRxTxLimit(float64(h.cfg.MinMBps), float64(h.cfg.MinMBps))
		h.l.Debug().Uint64("last_nic_rx_bytes", h.lastNicRxBytes).Uint64("last_nic_tx_bytes", h.lastNicTxBytes).Str("min", i2MBps(h.cfg.MinMBps)).Msg("last rx or tx is 0, use min limit")
		h.lastNicRxBytes = curNicRxBytes
		h.lastNicTxBytes = curNicTxBytes
		return
	}

	var (
		rxSub uint64
		txSub uint64
	)
	if curNicRxBytes < h.lastNicRxBytes {
		h.l.Warn().Uint64("last_nic_rx_bytes", h.lastNicRxBytes).Uint64("cur_nic_rx_bytes", curNicRxBytes).Msg("cur rx bytes is small than last rx bytes")
	} else {
		rxSub = curNicRxBytes - h.lastNicRxBytes
	}
	if curNicTxBytes < h.lastNicTxBytes {
		h.l.Warn().Uint64("last_nic_tx_bytes", h.lastNicTxBytes).Uint64("cur_nic_tx_bytes", curNicTxBytes).Msg("cur tx bytes is small than last tx bytes")
	} else {
		txSub = curNicTxBytes - h.lastNicTxBytes
	}

	rxMBps := float64(rxSub) / float64(1048576) / float64(h.cfg.MonitorInterval)
	txMBps := float64(txSub) / float64(1048576) / float64(h.cfg.MonitorInterval)
	rxLimit := calcLimit(rxMBps, float64(h.maxMBps), float64(h.cfg.MinMBps), float64(h.nicSpeedMBps), h.selfRxSpeedMBps)
	txLimit := calcLimit(txMBps, float64(h.maxMBps), float64(h.cfg.MinMBps), float64(h.nicSpeedMBps), h.selfTxSpeedMBps)
	h.l.Info().
		Str("nic_speed", i2MBps(h.nicSpeedMBps)).
		Str("rx_speed", f2MBps(rxMBps)).
		Str("tx_speed", f2MBps(txMBps)).
		Str("rx_limit", f2MBps(rxLimit)).
		Str("tx_limit", f2MBps(txLimit)).
		Str("max", i2MBps(h.maxMBps)).
		Str("min", i2MBps(h.cfg.MinMBps)).
		Uint64("nic_rx_bytes", curNicRxBytes).
		Uint64("nic_tx_bytes", curNicTxBytes).
		Msg("nic limit")
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
