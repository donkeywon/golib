package buildinfo

import (
	"github.com/earthboundkid/versioninfo/v2"
)

func init() {
	if Version == "" {
		Version = versioninfo.Version
	}
	if Revision == "" {
		Revision = versioninfo.Revision
	}
	if CommitTime == "" {
		CommitTime = versioninfo.LastCommit.Local().Format("2006-01-02 15:04:05")
	}
}

var (
	Version    = ""
	BuildTime  = ""
	CommitTime = ""
	Revision   = ""
)
