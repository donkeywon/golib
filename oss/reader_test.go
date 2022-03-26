package oss

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReader(t *testing.T) {
	r := NewReader(nil, testCfg())

	fp := "/tmp/test.file1"
	os.Remove(fp)
	f, _ := os.OpenFile(fp, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()

	nw, err := io.Copy(f, r)
	require.NoError(t, err)
	require.Equal(t, 1, int(nw))
}
