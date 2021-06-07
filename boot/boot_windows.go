package boot

import (
	"os"
)

var signals = []os.Signal{
	os.Interrupt,
}
