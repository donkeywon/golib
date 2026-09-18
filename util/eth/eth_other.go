//go:build !linux && !windows

package eth

import (
	"context"
	"errors"
)

var (
	ErrNicSpeedUnsupported = errors.New("nic speed is not supported on this platform")
)

func GetNicSpeed(ctx context.Context, nic string) (int, error) {
	return 0, ErrNicSpeedUnsupported
}
