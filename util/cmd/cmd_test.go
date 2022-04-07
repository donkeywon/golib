package cmd

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResultString(t *testing.T) {
	r := &Result{
		Stdout:        []string{"abc", "def"},
		Stderr:        []string{},
		ExitCode:      1,
		Pid:           123,
		StartTimeNano: 1234567890,
		StopTimeNano:  9876543210,
		Signaled:      true,
	}
	require.Equal(t, `{"stdout":["abc","def"],"stderr":[],"exitCode":1,"pid":123,"startTimeNano":1234567890,"stopTimeNano":9876543210,"signaled":true}`, r.String())
}
