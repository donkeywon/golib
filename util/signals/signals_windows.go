package signals

import (
	"os"
)

var (
	TermSignals = []os.Signal{
		os.Interrupt,
	}
	IntSignals = []os.Signal{
		os.Interrupt,
	}
)
