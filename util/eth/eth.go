package eth

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/prometheus/procfs"
)

// copied from github.com/prometheus/procfs/net_dev.go
type NetDevStats struct {
	Name         string `json:"name"`          // The name of the interface.
	RxBytes      uint64 `json:"rx_bytes"`      // Cumulative count of bytes received.
	RxPackets    uint64 `json:"rx_packets"`    // Cumulative count of packets received.
	RxErrors     uint64 `json:"rx_errors"`     // Cumulative count of receive errors encountered.
	RxDropped    uint64 `json:"rx_dropped"`    // Cumulative count of packets dropped while receiving.
	RxFIFO       uint64 `json:"rx_fifo"`       // Cumulative count of FIFO buffer errors.
	RxFrame      uint64 `json:"rx_frame"`      // Cumulative count of packet framing errors.
	RxCompressed uint64 `json:"rx_compressed"` // Cumulative count of compressed packets received by the device driver.
	RxMulticast  uint64 `json:"rx_multicast"`  // Cumulative count of multicast frames received by the device driver.
	TxBytes      uint64 `json:"tx_bytes"`      // Cumulative count of bytes transmitted.
	TxPackets    uint64 `json:"tx_packets"`    // Cumulative count of packets transmitted.
	TxErrors     uint64 `json:"tx_errors"`     // Cumulative count of transmit errors encountered.
	TxDropped    uint64 `json:"tx_dropped"`    // Cumulative count of packets dropped while transmitting.
	TxFIFO       uint64 `json:"tx_fifo"`       // Cumulative count of FIFO buffer errors.
	TxCollisions uint64 `json:"tx_collisions"` // Cumulative count of collisions detected on the interface.
	TxCarrier    uint64 `json:"tx_carrier"`    // Cumulative count of carrier losses detected by the device driver.
	TxCompressed uint64 `json:"tx_compressed"` // Cumulative count of compressed packets transmitted by the device driver.
}

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
