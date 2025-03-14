package oss

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiPartWriter(t *testing.T) {
	w := NewMultiPartWriter(context.TODO(), &Cfg{
		Ak:      "",
		Sk:      "",
		URL:     "",
		Retry:   1,
		Timeout: 1000,
		Region:  "",
	})

	f, _ := os.OpenFile("/tmp/test.file", os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()

	_, err := io.Copy(w, f)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}
