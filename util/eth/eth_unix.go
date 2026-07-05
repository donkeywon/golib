//go:build linux || darwin || freebsd || solaris

package eth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	ErrNegativeNicSpeed = errors.New("negative nic speed")
)

// GetNicSpeed in Mbps.
func GetNicSpeed(ctx context.Context, nic string) (int, error) {
	bs, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/speed", nic))
	if err != nil {
		return 0, err
	}

	speed, err := strconv.Atoi(strings.TrimSpace(string(bs)))
	if err != nil {
		return 0, err
	}

	if speed < 0 {
		return speed, ErrNegativeNicSpeed
	}

	return speed, nil
}
