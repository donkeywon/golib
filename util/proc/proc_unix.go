//go:build linux || darwin || freebsd || solaris

package proc

import (
	"context"
	"syscall"
	"time"
)

func Kill(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func KillGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

func MustKill(ctx context.Context, pid int, singleSigWaitExitSec int, sig ...syscall.Signal) error {
	if !Exists(pid) {
		return nil
	}

	for _, s := range sig {
		err := syscall.Kill(pid, s)
		if err != nil {
			return err
		}

		if WaitProcExit(ctx, pid, time.Second, singleSigWaitExitSec) {
			return nil
		}
	}

	return nil
}

func MustKillGroup(ctx context.Context, pid int, singleSigWaitExitSec int, sig ...syscall.Signal) error {
	if !Exists(pid) {
		return nil
	}

	for _, s := range sig {
		err := syscall.Kill(-pid, s)
		if err != nil {
			return err
		}

		if WaitProcExit(ctx, pid, time.Second, singleSigWaitExitSec) {
			return nil
		}
	}

	return nil
}
