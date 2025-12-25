//go:build windows || darwin || freebsd || solaris

package proc

import (
	"context"

	"github.com/shirou/gopsutil/v4/process"
)

func FindAllChildrenProcess(ctx context.Context, p *process.Process) ([]*process.Process, error) {
	children, err := p.ChildrenWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var allChildren []*process.Process
	for _, child := range children {
		allChildren = append(allChildren, child)
		subChildren, err := FindAllChildrenProcess(ctx, child)
		if err != nil {
			return nil, err
		}
		allChildren = append(allChildren, subChildren...)
	}

	return allChildren, nil
}

func FindAllChildrenPid(ctx context.Context, pid int) ([]int, error) {
	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return nil, err
	}

	children, err := FindAllChildrenProcess(ctx, p)
	if err != nil {
		return nil, err
	}

	if len(children) == 0 {
		return nil, nil
	}

	childrenPids := make([]int, 0, len(children))
	for _, child := range children {
		childrenPids = append(childrenPids, int(child.Pid))
	}

	return childrenPids, nil
}
