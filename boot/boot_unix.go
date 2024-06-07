//go:build linux || darwin || freebsd || solaris

package boot

import (
	"os"

	"golang.org/x/sys/unix"
)

var signals = []os.Signal{
	unix.SIGTERM,
	unix.SIGINT,
}
