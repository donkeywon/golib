package proc

import (
	"context"
	"os"
	"syscall"
	"time"
)

func Stop(pid int) error {
	if !Exists(pid) {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

func MustStop(_ context.Context, pid int) error {
	return Stop(pid)
}

func Kill(pid int, sig syscall.Signal) error {
	if !Exists(pid) {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

func KillGroup(pid int, sig syscall.Signal) error {
	return Kill(pid, sig)
}

func MustKill(ctx context.Context, pid int, singleSigWaitExitSec int, sig ...syscall.Signal) error {
	if !Exists(pid) {
		return nil
	}

	for _, s := range sig {
		err := Kill(pid, s)
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
		err := Kill(pid, s)
		if err != nil {
			return err
		}

		if WaitProcExit(ctx, pid, time.Second, singleSigWaitExitSec) {
			return nil
		}
	}

	return nil
}
