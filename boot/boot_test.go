package boot

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

func TestBoot(_ *testing.T) {
	go func() {
		time.Sleep(time.Second * 3)
		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	}()
	Boot()
}

func TestLoadCfg(t *testing.T) {
	type Cfg struct {
		Timeout time.Duration `yaml:"timeout"`
	}

	cfg := &Cfg{}
	content := []byte(`timeout: 1s`)
	err := yaml.UnmarshalWithOptions(content, cfg, yaml.CustomUnmarshaler(durationUnmarshaler))
	require.NoError(t, err)
}
