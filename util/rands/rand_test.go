package rands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandInt(t *testing.T) {
	t.Run("min less than max", func(t *testing.T) {
		for range 100 {
			v := RandInt(0, 10)
			assert.GreaterOrEqual(t, v, 0)
			assert.Less(t, v, 10)
		}
	})

	t.Run("min equals max", func(t *testing.T) {
		v := RandInt(5, 5)
		assert.Equal(t, 5, v)
	})

	t.Run("min greater than max", func(t *testing.T) {
		v := RandInt(10, 5)
		assert.Equal(t, 10, v)
	})

	t.Run("negative range", func(t *testing.T) {
		for range 50 {
			v := RandInt(-10, 0)
			assert.GreaterOrEqual(t, v, -10)
			assert.Less(t, v, 0)
		}
	})

	t.Run("value in range with multiple calls", func(t *testing.T) {
		// Run many times and verify all values are in range
		for range 1000 {
			v := RandInt(10, 20)
			assert.GreaterOrEqual(t, v, 10)
			assert.Less(t, v, 20)
		}
	})
}
