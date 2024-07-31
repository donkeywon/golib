package buildinfo

import "runtime"

var (
	Version   = ""
	BuildTime = ""
	GitCommit = ""
	GoVersion = runtime.Version()
	Arch      = runtime.GOARCH
)
