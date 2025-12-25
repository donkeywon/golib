package proc

import (
	"context"
	"errors"
	"math"
	"os"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sync/errgroup"
)

var MustKillSignals = []syscall.Signal{syscall.SIGINT, syscall.SIGKILL}

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

func GetSelfProcTree(ctx context.Context) ([]*process.Process, error) {
	all := make([]*process.Process, 0, 4)

	self, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if err != nil {
		return nil, err
	}
	all = append(all, self)

	allChildren, err := FindAllChildrenProcess(ctx, self)
	if err != nil {
		return nil, err
	}

	all = append(all, allChildren...)
	return all, nil
}

func CalcSelfCPUPercent(ctx context.Context, interval time.Duration) (float64, error) {
	return CalcProcCPUPercent(ctx, os.Getpid(), interval)
}

func CalcSelfProcTreeCPUPercent(ctx context.Context, interval time.Duration) (float64, error) {
	return CalcProcTreeCPUPercent(ctx, os.Getpid(), interval)
}

func CalcSelfMemoryUsage(ctx context.Context) (uint64, error) {
	return CalcProcMemoryUsage(ctx, os.Getpid())
}

func CalcSelfProcTreeMemoryUsage(ctx context.Context) (uint64, error) {
	return CalcProcTreeMemoryUsage(ctx, os.Getpid())
}

func CalcProcCPUPercent(ctx context.Context, pid int, interval time.Duration) (float64, error) {
	ps, err := pidsToProcess(ctx, true, pid)
	if err != nil {
		return 0, err
	}

	return CalcProcessesCPUPercent(ctx, interval, ps...)
}

func CalcProcTreeCPUPercent(ctx context.Context, pid int, interval time.Duration) (float64, error) {
	procTree, err := pidToProcessTree(ctx, true, pid)
	if err != nil {
		return 0, err
	}

	return CalcProcessesCPUPercent(ctx, interval, procTree...)
}

func CalcProcMemoryUsage(ctx context.Context, pid int) (uint64, error) {
	ps, err := pidsToProcess(ctx, true, pid)
	if err != nil {
		return 0, err
	}

	return CalcProcessesMemoryUsage(ps...)
}

func CalcProcTreeMemoryUsage(ctx context.Context, pid int) (uint64, error) {
	procTree, err := pidToProcessTree(ctx, true, pid)
	if err != nil {
		return 0, err
	}

	return CalcProcessesMemoryUsage(procTree...)
}

func CalcProcessesCPUPercent(ctx context.Context, interval time.Duration, ps ...*process.Process) (float64, error) {
	var totalCPU float64

	eg, ctx := errgroup.WithContext(ctx)
	for _, p := range ps {
		eg.Go(func() error {
			cpuPercent, err := p.PercentWithContext(ctx, interval)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}

			atomicAddFloat64(&totalCPU, cpuPercent)
			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		return 0, err
	}

	return totalCPU, nil
}

func CalcProcessesMemoryUsage(ps ...*process.Process) (uint64, error) {
	var totalMemory uint64
	for _, p := range ps {
		memInfo, err := p.MemoryInfo()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		totalMemory += memInfo.RSS
	}
	return totalMemory, nil
}

func atomicAddFloat64(addr *float64, delta float64) (newVal float64) {
	addrUint64 := (*uint64)(unsafe.Pointer(addr))
	for {
		oldBits := atomic.LoadUint64(addrUint64)
		oldVal := math.Float64frombits(oldBits)
		newVal = oldVal + delta
		newBits := math.Float64bits(newVal)
		if atomic.CompareAndSwapUint64(addrUint64, oldBits, newBits) {
			return newVal
		}
	}
}

func pidsToProcess(ctx context.Context, skipNotExists bool, pids ...int) ([]*process.Process, error) {
	if len(pids) == 0 {
		return nil, nil
	}

	ps := make([]*process.Process, 0, len(pids))
	for _, pid := range pids {
		p, err := process.NewProcessWithContext(ctx, int32(pid))
		if err != nil {
			if skipNotExists && errors.Is(err, process.ErrorProcessNotRunning) {
				continue
			}
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, nil
}

func pidToProcessTree(ctx context.Context, skipNotExists bool, pid int) ([]*process.Process, error) {
	children, err := FindAllChildrenPid(ctx, pid)
	if err != nil {
		return nil, err
	}

	all := make([]int, 0, 1+len(children))
	all = append(all, pid)
	all = append(all, children...)

	procTree, err := pidsToProcess(ctx, skipNotExists, all...)
	if err != nil {
		return nil, err
	}

	return procTree, nil
}
