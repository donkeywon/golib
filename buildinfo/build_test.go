package buildinfo

import (
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capture saves current state and returns a restore function.
func capture() func() {
	prevVersion := Version
	prevCommitTime := CommitTime
	prevDirtyBuild := DirtyBuild
	prevBuildTime := BuildTime
	prevRevision := Revision
	return func() {
		Version = prevVersion
		CommitTime = prevCommitTime
		DirtyBuild = prevDirtyBuild
		BuildTime = prevBuildTime
		Revision = prevRevision
	}
}

func TestApplyBuildInfo_NoInfo(t *testing.T) {
	restore := capture()
	defer restore()

	applyBuildInfo(nil, false)
	// All should remain at zero values
	// (They may already be set from init, so we verify no panic)
}

func TestApplyBuildInfo_EmptySettings(t *testing.T) {
	restore := capture()
	defer restore()

	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v1.0.0",
		},
		Settings: nil,
	}
	applyBuildInfo(info, true)
	assert.Equal(t, "v1.0.0", Version)
}

func TestApplyBuildInfo_WithSettings(t *testing.T) {
	restore := capture()
	defer restore()

	ts := "2024-01-15T10:00:00Z"
	parsedTime, err := time.Parse(time.RFC3339, ts)
	require.NoError(t, err)

	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v2.0.0",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123def"},
			{Key: "vcs.time", Value: ts},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	applyBuildInfo(info, true)

	assert.Equal(t, "v2.0.0", Version)
	assert.Equal(t, "abc123def", Revision)
	assert.True(t, parsedTime.Equal(CommitTime))
	assert.True(t, DirtyBuild)
}

func TestApplyBuildInfo_EmptyValue(t *testing.T) {
	restore := capture()
	defer restore()

	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v3.0.0",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: ""},      // empty, should be skipped
			{Key: "vcs.time", Value: ""},          // empty, should be skipped
			{Key: "vcs.modified", Value: "false"}, // non-empty, should be set
		},
	}
	applyBuildInfo(info, true)

	assert.Equal(t, "v3.0.0", Version)
	assert.Empty(t, Revision)
	assert.True(t, time.Time{}.Equal(CommitTime))
	assert.False(t, DirtyBuild)
}

func TestApplyBuildInfo_UnmatchedKeys(t *testing.T) {
	restore := capture()
	defer restore()

	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v4.0.0",
		},
		Settings: []debug.BuildSetting{
			{Key: "unknown.key", Value: "ignored"},
			{Key: "GOARCH", Value: "amd64"},
		},
	}
	applyBuildInfo(info, true)
	assert.Equal(t, "v4.0.0", Version)
	// These remain at their previous values
	assert.Empty(t, Revision)
	assert.True(t, time.Time{}.Equal(CommitTime))
	assert.False(t, DirtyBuild)
}

func TestBuildInfoVariables(t *testing.T) {
	// These variables are accessible and have types we can validate.
	t.Run("Version", func(t *testing.T) {
		assert.IsType(t, "", Version)
	})

	t.Run("CommitTime", func(t *testing.T) {
		assert.IsType(t, CommitTime, CommitTime)
	})

	t.Run("DirtyBuild", func(t *testing.T) {
		assert.IsType(t, false, DirtyBuild)
	})

	t.Run("BuildTime", func(t *testing.T) {
		assert.IsType(t, "", BuildTime)
	})

	t.Run("Revision", func(t *testing.T) {
		assert.IsType(t, "", Revision)
	})
}
