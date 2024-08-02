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
	profCh      = make(chan struct{})
	p           interface{ Stop() }
)

func Start(mode string, dir string, timeoutSec int) (string, error) {
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

	return filepath.Join(dir, filename), start(timeoutSec, opts...)
}

func start(timeoutSec int, options ...func(*profile.Profile)) error {
	if !atomic.CompareAndSwapUint32(&profSwitch, 0, 1) {
		return errors.New("already profiling")
	}

	opts := []func(*profile.Profile){profile.Quiet}

	p = profile.Start(append(opts, options...)...)

	if timeoutSec == 0 {
		timeoutSec = profTimeout
	}

	go func(timeoutSec int) {
		select {
		case <-time.After(time.Second * time.Duration(timeoutSec)):
			stop()
		case <-profCh:
		}
	}(timeoutSec)

	return nil
}

func Stop() error {
	if !atomic.CompareAndSwapUint32(&profSwitch, 1, 0) {
		return errors.New("not profiling")
	}
	stop()
	profCh <- struct{}{}
	return nil
}

func stop() {
	if p != nil {
		p.Stop()
	}
	atomic.StoreUint32(&profSwitch, 0)
}

func genDir(dir string, mode string) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%s-%s", time.Now().Format("20060102150405"), mode, uuid.NewString()))
}
