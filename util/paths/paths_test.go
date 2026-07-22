package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileExist(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(existingFile, []byte("test"), 0o644)
	assert.NoError(t, err)

	nonExistingFile := filepath.Join(tmpDir, "nonexistent.txt")

	t.Run("existing file", func(t *testing.T) {
		assert.True(t, FileExist(existingFile))
	})

	t.Run("non-existing file", func(t *testing.T) {
		assert.False(t, FileExist(nonExistingFile))
	})

	t.Run("directory is not a file", func(t *testing.T) {
		assert.False(t, FileExist(tmpDir))
	})
}

func TestDirExist(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistingDir := filepath.Join(tmpDir, "nonexistent")

	t.Run("existing directory", func(t *testing.T) {
		assert.True(t, DirExist(tmpDir))
	})

	t.Run("non-existing directory", func(t *testing.T) {
		assert.False(t, DirExist(nonExistingDir))
	})

	// Create a file and test DirExist on it
	existingFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(existingFile, []byte("test"), 0o644)
	assert.NoError(t, err)

	t.Run("file is not a directory", func(t *testing.T) {
		assert.False(t, DirExist(existingFile))
	})
}

func TestPathExist(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a temp file
	existingFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(existingFile, []byte("test"), 0o644)
	assert.NoError(t, err)

	nonExistingPath := filepath.Join(tmpDir, "nonexistent")

	t.Run("file path with isDir=false", func(t *testing.T) {
		assert.True(t, PathExist(existingFile, false))
	})

	t.Run("file path with isDir=true (mismatch)", func(t *testing.T) {
		assert.False(t, PathExist(existingFile, true))
	})

	t.Run("dir path with isDir=true", func(t *testing.T) {
		assert.True(t, PathExist(tmpDir, true))
	})

	t.Run("dir path with isDir=false (mismatch)", func(t *testing.T) {
		assert.False(t, PathExist(tmpDir, false))
	})

	t.Run("non-existing path with isDir=false", func(t *testing.T) {
		assert.False(t, PathExist(nonExistingPath, false))
	})

	t.Run("non-existing path with isDir=true", func(t *testing.T) {
		assert.False(t, PathExist(nonExistingPath, true))
	})
}
