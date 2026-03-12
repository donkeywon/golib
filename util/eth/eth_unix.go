//go:build linux || darwin || freebsd || solaris

package eth

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

// GetNetDevStats
// get statistics about nic.
func GetNetDevStats() (map[string]*NetDevStats, error) {
	fs, err := procfs.NewFS(procfs.DefaultMountPoint)
	if err != nil {
		return nil, errs.Wrap(err, "open proc fs failed")
	}

	netDev, err := fs.NetDev()
	if err != nil {
		return nil, errs.Wrap(err, "parse /proc/net/dev failed")
	}

	stats := make(map[string]*NetDevStats, len(netDev))
	for name, dev := range netDev {
		stats[name] = &NetDevStats{
			Name:         dev.Name,
			RxBytes:      dev.RxBytes,
			RxPackets:    dev.RxPackets,
			RxErrors:     dev.RxErrors,
			RxDropped:    dev.RxDropped,
			RxFIFO:       dev.RxFIFO,
			RxFrame:      dev.RxFrame,
			RxCompressed: dev.RxCompressed,
			RxMulticast:  dev.RxMulticast,
			TxBytes:      dev.TxBytes,
			TxPackets:    dev.TxPackets,
			TxErrors:     dev.TxErrors,
			TxDropped:    dev.TxDropped,
			TxFIFO:       dev.TxFIFO,
			TxCollisions: dev.TxCollisions,
			TxCarrier:    dev.TxCarrier,
			TxCompressed: dev.TxCompressed,
		}
	}

	return stats, nil
}
