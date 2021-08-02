package signals

import (
	"os"
)

var ExitSignals = []os.Signal{
	os.Interrupt,
}
