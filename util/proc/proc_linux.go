package proc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/conv"
	"github.com/shirou/gopsutil/v4/process"
)

func FindAllChildrenProcess(ctx context.Context, p *process.Process) ([]*process.Process, error) {
	childrenPids, err := FindAllChildrenPid(ctx, int(p.Pid))
	if err != nil {
		return nil, err
	}

	if len(childrenPids) == 0 {
		return nil, nil
	}

	children := make([]*process.Process, 0, len(childrenPids))
	for _, pid := range childrenPids {
		child, err := process.NewProcessWithContext(ctx, int32(pid))
		if err != nil {
			if errors.Is(err, process.ErrorProcessNotRunning) {
				continue
			}
			return nil, err
		}

		children = append(children, child)
	}

	return children, nil
}

func FindAllChildrenPid(ctx context.Context, pid int) ([]int, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	taskDir := fmt.Sprintf("/proc/%d/task", pid)
	taskDirs, err := os.ReadDir(taskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, syscall.ESRCH
		}

		return nil, errs.Wrapf(err, "read dir failed: %s", taskDir)
	}

	var children []int
	for _, taskDir := range taskDirs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !taskDir.IsDir() {
			continue
		}

		childrenFilepath := fmt.Sprintf("/proc/%d/task/%s/children", pid, taskDir.Name())
		childrenContent, err := os.ReadFile(childrenFilepath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, errs.Wrapf(err, "read file failed: %s", childrenFilepath)
		}

		if len(childrenContent) == 0 {
			continue
		}

		childrenPidStrs := strings.Fields(conv.Bytes2String(childrenContent))
		if len(childrenPidStrs) == 0 {
			continue
		}

		for _, childrenPidStr := range childrenPidStrs {
			cpid, err := strconv.Atoi(childrenPidStr)
			if err != nil {
				return nil, errs.Wrapf(err, "child pid is not number: %s", childrenPidStr)
			}
			children = append(children, cpid)

			children2, err := FindAllChildrenPid(ctx, cpid)
			if err != nil {
				return nil, err
			}
			if len(children2) == 0 {
				continue
			}
			children = append(children, children2...)
		}
	}

	return children, nil
}
