package wheeltimer

import (
	"runtime"
	"sync"
	"time"
)

type ringOption struct {
	waitStrategy WaitStrategy
}

func defaultRingOptions() *ringOption {
	return &ringOption{
		waitStrategy: NewYieldingWaitStrategy(),
	}
}

type RingOption func(r *ringOption)

// WithWaitStrategy sets the wait strategy for the ring buffer.
func WithWaitStrategy(strategy WaitStrategy) RingOption {
	return func(r *ringOption) {
		r.waitStrategy = strategy
	}
}

// WaitStrategy is a strategy for waiting on a sequence to be available.
type WaitStrategy interface {
	WaitFor(timeout time.Duration) error
	SignalAll()
}

// yieldingWaitStrategy is a strategy that uses a busy spin loop for waiting on a sequence to be available.
type yieldingWaitStrategy struct{}

func (s *yieldingWaitStrategy) WaitFor(timeout time.Duration) error {
	runtime.Gosched()
	return nil
}

func (s *yieldingWaitStrategy) SignalAll() {}

// sleepingWaitStrategy is a strategy that uses a Thread.Sleep(1) for waiting on a sequence to be available.
type sleepingWaitStrategy struct {
	sleepTime time.Duration
}

func (s *sleepingWaitStrategy) busySpinWaitFor(timeout time.Duration) error {
	now := time.Now()
	deadline := now.Add(timeout)
	target := now.Add(s.sleepTime)

	for {
		if time.Now().After(deadline) {
			return ErrTimeout
		}
		if time.Now().After(target) {
			return nil
		}
		runtime.Gosched()
	}
}

func (s *sleepingWaitStrategy) sleepWaitFor(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	remaining := time.Until(deadline)
	if remaining <= 0 {
		return ErrTimeout
	}

	waitTime := s.sleepTime
	if waitTime > remaining {
		waitTime = remaining
	}

	time.Sleep(waitTime)

	if time.Now().After(deadline) {
		return ErrTimeout
	}

	return nil
}

func (s *sleepingWaitStrategy) WaitFor(timeout time.Duration) error {
	if timeout <= 0 {
		time.Sleep(s.sleepTime)
		return nil
	}

	if s.sleepTime < time.Microsecond*100 {
		return s.busySpinWaitFor(timeout)
	}

	return s.sleepWaitFor(timeout)
}

func (s *sleepingWaitStrategy) SignalAll() {}

// BlockingWaitStrategy is a strategy that uses a sync.Cond for waiting on a sequence to be available.
type BlockingWaitStrategy struct {
	lock   sync.Mutex
	wakeCh chan struct{} // wake up channel, sync.Cond does not support timeout wait
}

func (s *BlockingWaitStrategy) WaitFor(timeout time.Duration) error {
	s.lock.Lock()
	ch := s.wakeCh
	s.lock.Unlock()

	if timeout <= 0 {
		<-ch
		return nil
	}

	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return ErrTimeout
	}
}

func (s *BlockingWaitStrategy) SignalAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	ch := s.wakeCh
	s.wakeCh = make(chan struct{})
	close(ch)
}

// NewYieldingWaitStrategy creates a new yielding wait strategy.
func NewYieldingWaitStrategy() WaitStrategy {
	return &yieldingWaitStrategy{}
}

// NewSleepingWaitStrategy creates a new sleeping wait strategy.
func NewSleepingWaitStrategy(sleepTime time.Duration) WaitStrategy {
	return &sleepingWaitStrategy{sleepTime: sleepTime}
}

// NewBlockingWaitStrategy creates a new blocking wait strategy.
func NewBlockingWaitStrategy() WaitStrategy {
	return &BlockingWaitStrategy{
		wakeCh: make(chan struct{}),
	}
}
