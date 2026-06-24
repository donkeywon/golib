package cmds

import (
	"os/exec"
	"syscall"
)

func SetPgid() Option {
	return func(cmd *exec.Cmd) {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.CreationFlags |= 0x00000200 // CREATE_NEW_PROCESS_GROUP
	}
}

func RunAsUser(u string) Option {
	return func(cmd *exec.Cmd) {
		// not supported
	}
}
