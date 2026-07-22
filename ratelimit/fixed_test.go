package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/donkeywon/golib/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFixedRateLimiterCfg(t *testing.T) {
	cfg := NewFixedCfg()
	require.NotNil(t, cfg)
	assert.Equal(t, 0, cfg.N)
	assert.Equal(t, 0, cfg.Burst)
}

func TestNewFixedRateLimiter(t *testing.T) {
	frl := NewFixed()
	require.NotNil(t, frl)
	assert.Nil(t, frl.rxRl)
	assert.Nil(t, frl.txRl)
}

func TestFixedRateLimiter_Init(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 100, Burst: 50}
		err := frl.Init(context.Background())
		require.NoError(t, err)
		require.NotNil(t, frl.rxRl)
		require.NotNil(t, frl.txRl)
	})

	t.Run("invalid N negative", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: -1, Burst: 10}
		err := frl.Init(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "N must ge 0")
	})

	t.Run("invalid Burst zero", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 10, Burst: 0}
		err := frl.Init(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Burst must gt 0")
	})

	t.Run("invalid Burst negative", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 10, Burst: -5}
		err := frl.Init(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Burst must gt 0")
	})

	t.Run("N zero and valid Burst", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 0, Burst: 1}
		err := frl.Init(context.Background())
		require.NoError(t, err)
	})
}

func TestFixed_SetCfg(t *testing.T) {
	frl := NewFixed()
	cfg := &FixedCfg{N: 100, Burst: 20}
	frl.SetCfg(cfg)
	assert.Equal(t, cfg, frl.cfg)

	// Type assertion panic on wrong type
	assert.Panics(t, func() {
		frl.SetCfg("not a FixedCfg")
	})
}

func TestFixed_RxWaitN(t *testing.T) {
	t.Run("n=0 returns nil", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 10, Burst: 10}
		require.NoError(t, frl.Init(context.Background()))

		err := frl.RxWaitN(context.Background(), 0, time.Second)
		require.NoError(t, err)
	})

	t.Run("n within burst", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 1000, Burst: 100}
		require.NoError(t, frl.Init(context.Background()))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for i := 0; i < 5; i++ {
			err := frl.RxWaitN(ctx, 1, 100*time.Millisecond)
			require.NoError(t, err)
		}
	})

	t.Run("timeout exceeded", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 1, Burst: 1}
		require.NoError(t, frl.Init(context.Background()))

		// Consume the allowance
		err := frl.RxWaitN(context.Background(), 1, 0)
		require.NoError(t, err)

		// Next call should timeout quickly
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		err = frl.RxWaitN(ctx, 1, 10*time.Millisecond)
		require.Error(t, err)
	})

	t.Run("timeout=0 waits indefinitely", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 1000, Burst: 100}
		require.NoError(t, frl.Init(context.Background()))

		// With a high rate limit, timeout=0 should work fine
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := frl.RxWaitN(ctx, 1, 0)
		require.NoError(t, err)
	})

	t.Run("large n", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 1000, Burst: 20}
		require.NoError(t, frl.Init(context.Background()))

		// Large n that exceeds burst will still work eventually
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := frl.RxWaitN(ctx, 5, 5*time.Second)
		require.NoError(t, err)
	})
}

func TestFixed_TxWaitN(t *testing.T) {
	t.Run("n=0 returns nil", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 10, Burst: 10}
		require.NoError(t, frl.Init(context.Background()))

		err := frl.TxWaitN(context.Background(), 0, time.Second)
		require.NoError(t, err)
	})

	t.Run("n within burst", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 1000, Burst: 100}
		require.NoError(t, frl.Init(context.Background()))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for i := 0; i < 5; i++ {
			err := frl.TxWaitN(ctx, 1, 100*time.Millisecond)
			require.NoError(t, err)
		}
	})

	t.Run("timeout exceeded", func(t *testing.T) {
		frl := NewFixed()
		frl.cfg = &FixedCfg{N: 1, Burst: 1}
		require.NoError(t, frl.Init(context.Background()))

		err := frl.TxWaitN(context.Background(), 1, 0)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		err = frl.TxWaitN(ctx, 1, 10*time.Millisecond)
		require.Error(t, err)
	})
}

func TestFixed_SetRxLimit(t *testing.T) {
	frl := NewFixed()
	frl.cfg = &FixedCfg{N: 100, Burst: 10}
	require.NoError(t, frl.Init(context.Background()))

	frl.SetRxLimit(50, 5)
	// After setting limit, bursts should work
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := frl.RxWaitN(ctx, 1, 100*time.Millisecond)
	require.NoError(t, err)
}

func TestFixed_SetTxLimit(t *testing.T) {
	frl := NewFixed()
	frl.cfg = &FixedCfg{N: 100, Burst: 10}
	require.NoError(t, frl.Init(context.Background()))

	frl.SetTxLimit(50, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := frl.TxWaitN(ctx, 1, 100*time.Millisecond)
	require.NoError(t, err)
}

func TestFixed_PluginReg(t *testing.T) {
	// The init() should have registered the "fixed" type.
	// Verify by creating via the plugin system.
	rl := plugin.CreateWithCfg[RxTxRateLimiter](TypeFixed, NewFixedCfg())
	require.NotNil(t, rl)
	_, ok := rl.(*Fixed)
	assert.True(t, ok, "plugin should create a *Fixed")
}

func TestFixed_WaitN_NoTimeout(t *testing.T) {
	frl := NewFixed()
	frl.cfg = &FixedCfg{N: 1000, Burst: 100}
	require.NoError(t, frl.Init(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// timeout=0 means no timeout from the ratelimiter side, but context still has timeout
	err := frl.RxWaitN(ctx, 1, 0)
	require.NoError(t, err)
}
