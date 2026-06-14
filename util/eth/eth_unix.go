//go:build linux || darwin || freebsd || solaris

package eth

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/donkeywon/golib/errs"
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
