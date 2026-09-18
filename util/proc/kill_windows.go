package proc

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

func Kill(pid int, sig syscall.Signal) error {
	exists, err := Exists(pid)
	if err != nil {
		return err
	}
	if !exists {
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

func MustKill(ctx context.Context, pid int, interval time.Duration, count int, sig ...syscall.Signal) error {
	exists, err := Exists(pid)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	for _, s := range sig {
		err := Kill(pid, s)
		if err != nil {
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
		err := Kill(pgid, s)
		if err != nil {
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
