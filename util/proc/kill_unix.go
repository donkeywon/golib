//go:build linux || darwin || freebsd || solaris

package proc

import (
	"context"
	"errors"
	"syscall"
	"time"
)

func Kill(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func KillGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

func MustKill(ctx context.Context, pid int, interval time.Duration, count int, sig ...syscall.Signal) error {
	exists, err := Exists(pid)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	for _, s := range sig {
		err := syscall.Kill(pid, s)
		if err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return err
		}

		err = WaitProcExit(ctx, pid, interval, count)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrProcessNotExit) {
			return err
		}
	}

	return ErrProcessNotExit
}

func MustKillGroup(ctx context.Context, pgid int, interval time.Duration, count int, sig ...syscall.Signal) error {
	exists, err := Exists(pgid)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	for _, s := range sig {
		err := syscall.Kill(-pgid, s)
		if err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return err
		}

		err = WaitProcExit(ctx, pgid, interval, count)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrProcessNotExit) {
			return err
		}
	}

	return ErrProcessNotExit
}
