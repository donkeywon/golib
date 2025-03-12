package oss

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReader(t *testing.T) {
	r := NewReader(nil, &Cfg{
		URL:     "",
		Retry:   3,
		Timeout: 10,
		Ak:      "",
		Sk:      "",
		Region:  "local",
	})

	f, _ := os.OpenFile("/tmp/test.file", os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()

	nw, err := io.Copy(f, r)
	require.NoError(t, err)
	require.Equal(t, 1, int(nw))
}
