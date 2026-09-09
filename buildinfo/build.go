package buildinfo

import (
	"runtime/debug"
	"time"
)

func init() {
	applyBuildInfo(debug.ReadBuildInfo())
}

func applyBuildInfo(info *debug.BuildInfo, ok bool) {
	if !ok {
		return
	}
	Version = info.Main.Version
	for _, kv := range info.Settings {
		if kv.Value == "" {
			continue
		}
		switch kv.Key {
		case "vcs.revision":
			Revision = kv.Value
		case "vcs.time":
			CommitTime, _ = time.Parse(time.RFC3339, kv.Value)
		case "vcs.modified":
			DirtyBuild = kv.Value == "true"
		}
	}
}

var (
	DirtyBuild bool
	CommitTime time.Time
	Version    string
	BuildTime  string
	Revision   string
)
