//go:build linux || darwin || freebsd || solaris

package proc

import (
	"context"
	"syscall"
	"time"
)

func Stop(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func MustStop(ctx context.Context, pid int) error {
	if !Exists(pid) {
		return nil
	}

	err := syscall.Kill(pid, syscall.SIGTERM)
	if err != nil {
		return err
	}

	if WaitProcExit(ctx, pid, time.Second, mustStopWaitSec) {
		return nil
	}

	err = syscall.Kill(pid, syscall.SIGINT)
	if err != nil {
		return err
	}

	if WaitProcExit(ctx, pid, time.Second, mustStopWaitSec) {
		return nil
	}

	err = syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		return err
	}

	WaitProcExit(ctx, pid, time.Second, mustStopWaitSec)
	return nil
}
