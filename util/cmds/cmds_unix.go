//go:build linux || darwin || freebsd || solaris

package cmds

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func SetPgid() Option {
	return func(cmd *exec.Cmd) {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Setpgid = true
	}
}

func RunAsUser(u string) Option {
	return func(cmd *exec.Cmd) {
		u, err := user.Lookup(u)
		if err != nil {
			panic(err)
		}

		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			panic(fmt.Errorf("user: uid is not decimal: %s", u.Uid))
		}
		gid, err := strconv.Atoi(u.Gid)
		if err != nil {
			panic(fmt.Errorf("group: gid is not decimal: %s", u.Gid))
		}

		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}

		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
}
