package proc

import (
	"context"
	"os"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
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

func GetSelfAndAllChildrenProcess() ([]*process.Process, error) {
	all := make([]*process.Process, 0, 4)

	self, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return nil, err
	}
	all = append(all, self)

	allChildren, err := ListAllChildrenProcess(self)
	if err != nil {
		return nil, err
	}

	all = append(all, allChildren...)
	return all, nil
}

func ListAllChildrenProcess(p *process.Process) ([]*process.Process, error) {
	children, err := p.Children()
	if err != nil {
		return nil, err
	}

	var allChildren []*process.Process
	for _, child := range children {
		allChildren = append(allChildren, child)
		subChildren, err := ListAllChildrenProcess(child)
		if err != nil {
			return nil, err
		}
		allChildren = append(allChildren, subChildren...)
	}

	return allChildren, nil
}

func CalcSelfCPUPercent() (float64, error) {
	self, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return 0, err
	}
	return CalcCPUPercent([]*process.Process{self})
}

func CalcSelfAndAllChildrenCPUPercent() (float64, error) {
	ps, err := GetSelfAndAllChildrenProcess()
	if err != nil {
		return 0, err
	}
	return CalcCPUPercent(ps)
}

func CalcSelfMemoryUsage() (uint64, error) {
	self, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return 0, err
	}
	return CalcMemoryUsage([]*process.Process{self})
}

func CalcSelfAndAllChildrenMemoryUsage() (uint64, error) {
	ps, err := GetSelfAndAllChildrenProcess()
	if err != nil {
		return 0, err
	}
	return CalcMemoryUsage(ps)
}

func CalcCPUPercent(ps []*process.Process) (float64, error) {
	var totalCPU float64
	for _, p := range ps {
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			return 0, err
		}
		totalCPU += cpuPercent
	}
	return totalCPU, nil
}

func CalcMemoryUsage(ps []*process.Process) (uint64, error) {
	var totalMemory uint64
	for _, p := range ps {
		memInfo, err := p.MemoryInfo()
		if err != nil {
			return 0, err
		}
		totalMemory += memInfo.RSS
	}
	return totalMemory, nil
}
