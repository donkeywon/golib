package upd

import (
	"testing"

	"github.com/donkeywon/golib/pipeline"
	"github.com/stretchr/testify/require"
)

func TestDownloadPackage(t *testing.T) {
	err := downloadPackage("/tmp", "logagent.tar.gz", 1024*1024, &pipeline.StoreCfg{
		Type:     pipeline.StoreTypeOss,
		Cfg:      &pipeline.OssCfg{},
		URL:      "https://mirrors.midea.com/datalog/logagent-1.8.4.2.linux-amd64.tar.gz",
		Checksum: "903bf095bd0b9476aa612ca3e8bc4a3be5377950",
		Retry:    3,
		Timeout:  300,
	}, "sha1")
	require.NoError(t, err)
}
