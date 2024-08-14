package prof

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/profile"
)

const (
	defaultDir = "/tmp"
)

var (
	profSwitch  uint32
	profTimeout = 300
	stopCh      = make(chan struct{})
	p           interface{ Stop() }
)

func Start(mode string, dir string, timeoutSec int) (string, <-chan struct{}, error) {
	opts := []func(*profile.Profile){}

	filename := mode + ".pprof"
	if mode != "" {
		switch mode {
		case "cpu":
			opts = append(opts, profile.CPUProfile)
		case "mem":
			opts = append(opts, profile.MemProfile)
		case "heap":
			opts = append(opts, profile.MemProfileHeap)
		case "allocs":
			opts = append(opts, profile.MemProfileAllocs)
		case "mutex":
			opts = append(opts, profile.MutexProfile)
		case "block":
			opts = append(opts, profile.BlockProfile)
		case "trace":
			opts = append(opts, profile.TraceProfile)
			filename = "trace.out"
		case "thread":
			opts = append(opts, profile.ThreadcreationProfile)
		case "goroutine":
			opts = append(opts, profile.GoroutineProfile)
		case "clock":
			opts = append(opts, profile.ClockProfile)
		default:
			opts = append(opts, profile.CPUProfile)
			filename = "cpu.pprof"
		}
	}

	if dir != "" {
		dir = genDir(dir, mode)
	} else {
		dir = genDir(defaultDir, mode)
	}
	opts = append(opts, profile.ProfilePath(dir))

	done, err := start(timeoutSec, opts...)
	return filepath.Join(dir, filename), done, err
}

func start(timeoutSec int, options ...func(*profile.Profile)) (<-chan struct{}, error) {
	if !atomic.CompareAndSwapUint32(&profSwitch, 0, 1) {
		return nil, errors.New("already profiling")
	}

	opts := []func(*profile.Profile){profile.Quiet}

	p = profile.Start(append(opts, options...)...)

	if timeoutSec <= 0 {
		timeoutSec = profTimeout
	}

	done := make(chan struct{})
	go func(timeoutSec int) {
		t := time.NewTimer(time.Second * time.Duration(timeoutSec))
		defer t.Stop()
		select {
		case <-t.C:
		case <-stopCh:
		}
		p.Stop()
		p = nil
		atomic.StoreUint32(&profSwitch, 0)
		close(done)
	}(timeoutSec)

	return done, nil
}

func Stop() error {
	if atomic.LoadUint32(&profSwitch) != 1 {
		return errors.New("not profiling")
	}
	stopCh <- struct{}{}
	return nil
}

func genDir(dir string, mode string) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%s-%s", time.Now().Format("20060102150405"), mode, uuid.NewString()))
}
