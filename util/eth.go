package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/prometheus/procfs"
)

// GetNicSpeed
// get nic speed in Mbps.
func GetNicSpeed(nic string) (int, error) {
	bs, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/speed", nic))
	if err != nil {
		return 0, err
	}

	speed, err := strconv.Atoi(strings.TrimSpace(string(bs)))
	if err != nil {
		return 0, err
	}

	if speed < 0 {
		return speed, errs.New("nic speed is smaller than 0")
	}

	return speed, nil
}

// NetDevStats
// get statistics about nic
// 根据网卡名获取网卡运行状态，用途之一是用来计算RX和TX速度，支持获取多个nic状态.
func NetDevStats(nic ...string) (map[string]map[string]uint64, error) {
	fs, err := procfs.NewFS(procfs.DefaultMountPoint)
	if err != nil {
		return nil, errs.Wrap(err, "open proc fs fail")
	}

	netDev, err := fs.NetDev()
	if err != nil {
		return nil, errs.Wrap(err, "parse /proc/net/dev fail")
	}

	metrics := make(map[string]map[string]uint64)

	for _, stats := range netDev {
		if !Contains(nic, stats.Name) {
			continue
		}

		metrics[stats.Name] = map[string]uint64{
			"receive_bytes":       stats.RxBytes,
			"receive_packets":     stats.RxPackets,
			"receive_errors":      stats.RxErrors,
			"receive_dropped":     stats.RxDropped,
			"receive_fifo":        stats.RxFIFO,
			"receive_frame":       stats.RxFrame,
			"receive_compressed":  stats.RxCompressed,
			"receive_multicast":   stats.RxMulticast,
			"transmit_bytes":      stats.TxBytes,
			"transmit_packets":    stats.TxPackets,
			"transmit_errors":     stats.TxErrors,
			"transmit_dropped":    stats.TxDropped,
			"transmit_fifo":       stats.TxFIFO,
			"transmit_colls":      stats.TxCollisions,
			"transmit_carrier":    stats.TxCarrier,
			"transmit_compressed": stats.TxCompressed,
		}
	}

	return metrics, nil
}
