package proc

import (
	"context"
	"os"
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
