package proc

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

const mustStopWaitSec = 60

func Exists(pid int) bool {
	exists, _ := process.PidExists(int32(pid))
	return exists
}

func WaitProcExit(ctx context.Context, pid int, interval time.Duration, count int) bool {
	if !Exists(pid) {
		return true
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	inf := false
	if count == 0 {
		inf = true
	}

	for {
		select {
		case <-ticker.C:
			if !Exists(pid) {
				return true
			}
		case <-ctx.Done():
			return false
		}

		if !inf {
			count--
			if count <= 0 {
				return false
			}
		}
	}
}
