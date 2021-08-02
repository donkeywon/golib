//go:build linux || darwin || freebsd || solaris

package signals

import (
	"os"

	"golang.org/x/sys/unix"
)

var ExitSignals = []os.Signal{
	unix.SIGTERM,
	unix.SIGINT,
}
