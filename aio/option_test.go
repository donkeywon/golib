package aio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewOption(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		o := newOption()
		assert.Equal(t, 1024*1024, o.bufSize)
		assert.Equal(t, 0, o.queueSize)
		assert.Equal(t, time.Duration(0), o.deadline)
		assert.Equal(t, 0, o.deadlineFlushMinSize)
	})
}

func TestOption_apply(t *testing.T) {
	t.Run("applies option to target", func(t *testing.T) {
		o := newOption()
		opt := BufSize(4096)
		opt.apply(o)
		assert.Equal(t, 4096, o.bufSize)
	})
}

func TestBufSize(t *testing.T) {
	t.Run("positive value sets bufSize", func(t *testing.T) {
		o := newOption()
		opt := BufSize(4096)
		opt.apply(o)
		assert.Equal(t, 4096, o.bufSize)
	})

	t.Run("zero value is ignored", func(t *testing.T) {
		o := newOption()
		opt := BufSize(0)
		opt.apply(o)
		assert.Equal(t, 1024*1024, o.bufSize)
	})

	t.Run("negative value is ignored", func(t *testing.T) {
		o := newOption()
		opt := BufSize(-1)
		opt.apply(o)
		assert.Equal(t, 1024*1024, o.bufSize)
	})
}

func TestQueueSize(t *testing.T) {
	t.Run("positive value sets queueSize", func(t *testing.T) {
		o := newOption()
		opt := QueueSize(100)
		opt.apply(o)
		assert.Equal(t, 100, o.queueSize)
	})

	t.Run("zero value is ignored", func(t *testing.T) {
		o := newOption()
		opt := QueueSize(0)
		opt.apply(o)
		assert.Equal(t, 0, o.queueSize)
	})

	t.Run("negative value is ignored", func(t *testing.T) {
		o := newOption()
		opt := QueueSize(-5)
		opt.apply(o)
		assert.Equal(t, 0, o.queueSize)
	})
}

func TestDeadline(t *testing.T) {
	t.Run("positive duration sets deadline", func(t *testing.T) {
		o := newOption()
		opt := Deadline(5 * time.Second)
		opt.apply(o)
		assert.Equal(t, 5*time.Second, o.deadline)
	})

	t.Run("zero duration is ignored", func(t *testing.T) {
		o := newOption()
		opt := Deadline(0)
		opt.apply(o)
		assert.Equal(t, time.Duration(0), o.deadline)
	})

	t.Run("negative duration is ignored", func(t *testing.T) {
		o := newOption()
		opt := Deadline(-time.Second)
		opt.apply(o)
		assert.Equal(t, time.Duration(0), o.deadline)
	})
}

func TestDeadlineFlushMinSize(t *testing.T) {
	t.Run("positive value sets deadlineFlushMinSize", func(t *testing.T) {
		o := newOption()
		opt := DeadlineFlushMinSize(1024)
		opt.apply(o)
		assert.Equal(t, 1024, o.deadlineFlushMinSize)
	})

	t.Run("zero value is ignored", func(t *testing.T) {
		o := newOption()
		opt := DeadlineFlushMinSize(0)
		opt.apply(o)
		assert.Equal(t, 0, o.deadlineFlushMinSize)
	})

	t.Run("negative value is ignored", func(t *testing.T) {
		o := newOption()
		opt := DeadlineFlushMinSize(-1)
		opt.apply(o)
		assert.Equal(t, 0, o.deadlineFlushMinSize)
	})
}

func TestOptionComposition(t *testing.T) {
	t.Run("multiple options compose correctly", func(t *testing.T) {
		o := newOption()
		opts := []Option{
			BufSize(4096),
			QueueSize(64),
			Deadline(10 * time.Second),
			DeadlineFlushMinSize(512),
		}
		for _, opt := range opts {
			opt.apply(o)
		}
		assert.Equal(t, 4096, o.bufSize)
		assert.Equal(t, 64, o.queueSize)
		assert.Equal(t, 10*time.Second, o.deadline)
		assert.Equal(t, 512, o.deadlineFlushMinSize)
	})
}
