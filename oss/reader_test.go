package oss

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewReader(t *testing.T) {
	srv, _ := newFakeS3Server(t)
	defer srv.Close()
	cfg := fakeS3Cfg(srv, "reader-key")
	r := NewReader(context.Background(), cfg)
	require.NotNil(t, r)
	require.NotNil(t, r.Reader)
	r.Close()
}

func TestReader_Read(t *testing.T) {
	srv, backend := newFakeS3Server(t)
	defer srv.Close()

	backend.PutObject("test-bucket", "test-key", nil,
		strings.NewReader(testContent), int64(len(testContent)), nil)

	cfg := fakeS3Cfg(srv, "test-key")
	r := NewReader(context.Background(), cfg)
	defer r.Close()

	buf := make([]byte, 5)
	nr, err := r.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 5, nr)
}

func TestReader_ReadAll(t *testing.T) {
	srv, backend := newFakeS3Server(t)
	defer srv.Close()

	backend.PutObject("test-bucket", "test-key", nil,
		strings.NewReader(testContent), int64(len(testContent)), nil)

	cfg := fakeS3Cfg(srv, "test-key")
	r := NewReader(context.Background(), cfg)
	defer r.Close()

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, testContent, string(data))
}
