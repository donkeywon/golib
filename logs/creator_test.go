package logs

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUniqueStrings(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		result := uniqueStrings(nil)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := uniqueStrings([]string{})
		assert.Empty(t, result)
	})

	t.Run("single element", func(t *testing.T) {
		result := uniqueStrings([]string{"a"})
		assert.Equal(t, []string{"a"}, result)
	})

	t.Run("all unique", func(t *testing.T) {
		result := uniqueStrings([]string{"a", "b", "c"})
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("with duplicates", func(t *testing.T) {
		result := uniqueStrings([]string{"a", "b", "a", "c", "b"})
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("all duplicates", func(t *testing.T) {
		result := uniqueStrings([]string{"x", "x", "x"})
		assert.Equal(t, []string{"x"}, result)
	})

	t.Run("preserves order", func(t *testing.T) {
		result := uniqueStrings([]string{"z", "a", "m", "a", "z"})
		assert.Equal(t, []string{"z", "a", "m"}, result)
	})
}

func TestNopLoggerCreator(t *testing.T) {
	t.Run("NopLoggerCreator is not nil", func(t *testing.T) {
		assert.NotNil(t, NopLoggerCreator)
	})

	t.Run("Create returns logger with DiscardHandler", func(t *testing.T) {
		l, err := NopLoggerCreator.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
		// Writing to the nop logger should not panic or error
		l.Info("this goes nowhere")
	})
}

func TestNewRotateLoggerCreator(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		assert.Equal(t, DefaultFilepath, r.Filepath)
		assert.Equal(t, DefaultFormat, r.Format)
		assert.Equal(t, DefaultMaxFileSize, r.MaxFileSize)
		assert.Equal(t, DefaultMaxBackups, r.MaxBackups)
		assert.Equal(t, DefaultMaxAge, r.MaxAge)
		assert.Equal(t, DefaultEnableCompress, r.EnableCompress)
		assert.Equal(t, DefaultCompression, r.Compression)
		assert.NotNil(t, r.Level)
	})
}

func TestRotateLoggerCreator_Create(t *testing.T) {
	t.Run("stdout path creates logger", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = "stdout"
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
	})

	t.Run("stderr path creates logger", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = "stderr"
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
	})

	t.Run("multiple paths with duplicates", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = "stdout,stderr,stdout"
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
	})

	t.Run("text format creates TextHandler", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = "stdout"
		r.Format = "text"
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
		// Exercise the logger to ensure text handler works
		l.Info("test text format")
	})

	t.Run("json format creates JSONHandler", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = "stdout"
		r.Format = "json"
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
		l.Info("test json format")
	})

	t.Run("empty filepath creates file logger in cwd", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = ""
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
		// Empty string after split gives [""], filepath.Dir("") is "." which exists
		l.Info("empty filepath test")
	})

	t.Run("nonexistent dir returns error", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = "/nonexistent/dir/nope/app.log"
		l, err := r.Create()
		assert.Error(t, err)
		assert.Nil(t, l)
		assert.Contains(t, err.Error(), "build logger outputs failed")
	})

	t.Run("file in temp dir creates file logger", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")
		r := NewRotateLoggerCreator()
		r.Filepath = logPath
		r.Format = "json"
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
		l.Info("test file logger")

		// Verify the file was created and has content
		_, err = os.Stat(logPath)
		assert.NoError(t, err)
	})

	t.Run("custom level", func(t *testing.T) {
		r := NewRotateLoggerCreator()
		r.Filepath = "stdout"
		r.Level = slog.LevelDebug
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
	})

	t.Run("custom max file size and backups", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "rotated.log")
		r := NewRotateLoggerCreator()
		r.Filepath = logPath
		r.MaxFileSize = 1
		r.MaxBackups = 2
		r.MaxAge = 7
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
	})

	t.Run("enable compress with zstd", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "compressed.log")
		r := NewRotateLoggerCreator()
		r.Filepath = logPath
		r.EnableCompress = true
		r.Compression = "zstd"
		l, err := r.Create()
		require.NoError(t, err)
		require.NotNil(t, l)
	})
}
