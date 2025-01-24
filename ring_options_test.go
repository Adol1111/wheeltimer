package wheeltimer

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitStrategy(t *testing.T) {
	t.Run("YieldingWaitStrategy", func(t *testing.T) {
		s := NewYieldingWaitStrategy()
		err := s.WaitFor(time.Millisecond)
		assert.NoError(t, err)
	})

	t.Run("SleepingWaitStrategy", func(t *testing.T) {
		s := NewSleepingWaitStrategy(time.Millisecond * 10)
		err := s.WaitFor(time.Millisecond * 20)
		assert.NoError(t, err)

		err = s.WaitFor(time.Millisecond * 5)
		assert.ErrorIs(t, err, ErrTimeout)
	})

	t.Run("BusySpinWaitStrategy", func(t *testing.T) {
		s := NewSleepingWaitStrategy(time.Microsecond)
		now := time.Now()
		err := s.WaitFor(time.Millisecond)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(now).Microseconds(), int64(1))
	})

	t.Run("BlockingWaitStrategy", func(t *testing.T) {
		s := NewBlockingWaitStrategy()
		err := s.WaitFor(time.Millisecond)
		assert.ErrorIs(t, err, ErrTimeout)

		var closed atomic.Bool
		go func() {
			time.Sleep(time.Millisecond * 10)
			s.SignalAll()
			closed.Store(true)
		}()

		err = s.WaitFor(0)
		assert.NoError(t, err)
		assert.True(t, closed.Load())
	})
}

func BenchmarkTestWaitStrategy(b *testing.B) {
	b.Run("YieldingWaitStrategy", func(b *testing.B) {
		b.ReportAllocs()
		s := NewYieldingWaitStrategy()
		for i := 0; i < b.N; i++ {
			_ = s.WaitFor(time.Millisecond)
		}
	})

	b.Run("SleepingWaitStrategy", func(b *testing.B) {
		b.ReportAllocs()
		s := NewSleepingWaitStrategy(time.Millisecond)
		for i := 0; i < b.N; i++ {
			_ = s.WaitFor(time.Millisecond)
		}
	})

	b.Run("BusySpinWaitStrategy", func(b *testing.B) {
		b.ReportAllocs()
		s := NewSleepingWaitStrategy(time.Microsecond)
		for i := 0; i < b.N; i++ {
			_ = s.WaitFor(time.Millisecond)
		}
	})

	b.Run("BlockingWaitStrategy", func(b *testing.B) {
		b.ReportAllocs()
		s := NewBlockingWaitStrategy()
		for i := 0; i < b.N; i++ {
			_ = s.WaitFor(time.Millisecond)
		}
	})
}
