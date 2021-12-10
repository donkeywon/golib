//go:build linux || darwin || freebsd || solaris

package signals

import (
	"os"

	"golang.org/x/sys/unix"
)

var (
	TermSignals = []os.Signal{
		unix.SIGTERM,
	}
	IntSignals = []os.Signal{
		unix.SIGINT,
	}
)
