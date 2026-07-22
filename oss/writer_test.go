package oss

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAppendWriter(t *testing.T) {
	srv, _ := newFakeS3Server(t)
	defer srv.Close()
	cfg := fakeS3Cfg(srv, "append-key")
	w := NewAppendWriter(context.Background(), cfg)
	require.NotNil(t, w)
	w.Close()
}

func TestNewMultiPartWriter(t *testing.T) {
	srv, _ := newFakeS3Server(t)
	defer srv.Close()
	cfg := fakeS3Cfg(srv, "mp-key")
	w := NewMultiPartWriter(context.Background(), cfg)
	require.NotNil(t, w)
	w.Close()
}
