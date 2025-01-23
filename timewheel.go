package wheeltimer

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adol1111/wheeltimer/utils"
)

type workerState int32

const (
	workerStateInit workerState = iota
	workerStateStarted
	workerStateShutdown
)

type timeoutState int32

const (
	timeoutStateInit timeoutState = iota
	timeoutStateCancelled
	timeoutStateExpired
)

const (
	MPSC_CHUNK_SIZE = 1024
)

type WheelTimer struct {
	*option

	tickDuration time.Duration
	wheel        []*WheelBucket
	mask         int
	tick         int

	workerState          atomic.Int32
	startTime            atomic.Value
	startTimeInitializer sync.WaitGroup

	timeouts          *RingBuffer
	cancelledTimeouts *RingBuffer

	unprocessedTimeouts []*WheelTimeout
	pendingTimeouts     atomic.Int64

	closedCh chan struct{}
}

func NewWheelTimer(tickDuration time.Duration, ticksPerWheel uint32, opts ...WheelTimerOption) (*WheelTimer, error) {
	o := newDefaultOption()
	for _, opt := range opts {
		opt(o)
	}

	wheel := newTimerWheel(ticksPerWheel)
	mask := len(wheel) - 1

	maxDuration := math.MaxInt64 / int64(len(wheel))

	if int64(tickDuration) > maxDuration {
		return nil, fmt.Errorf("tickDuration: %d (expected: 0 < tickDuration in nanos < %d", tickDuration, maxDuration)
	}

	if tickDuration < time.Millisecond {
		o.logger.Warn(fmt.Sprintf("Configured tickDuration %s smaller then 1ms, using 1ms instead", tickDuration.String()))
		tickDuration = time.Millisecond
	}

	wt := &WheelTimer{
		tickDuration:      tickDuration,
		wheel:             wheel,
		mask:              mask,
		timeouts:          NewRingBuffer(MPSC_CHUNK_SIZE),
		cancelledTimeouts: NewRingBuffer(MPSC_CHUNK_SIZE),
		option:            o,
		closedCh:          make(chan struct{}),
	}
	wt.startTimeInitializer.Add(1)

	return wt, nil
}

func (tw *WheelTimer) Start() error {
	switch workerState(tw.workerState.Load()) {
	case workerStateInit:
		if tw.workerState.CompareAndSwap(int32(workerStateInit), int32(workerStateStarted)) {
			go tw.run()
		}
	case workerStateStarted:
		break
	case workerStateShutdown:
		return fmt.Errorf("cannot be started once stopped")
	default:
		return fmt.Errorf("invalid worker state")
	}

	if t, ok := tw.startTime.Load().(time.Time); !ok || t.IsZero() {
		tw.startTimeInitializer.Wait()
	}
	return nil
}

func (tw *WheelTimer) Stop() []Timeout {
	if !tw.workerState.CompareAndSwap(int32(workerStateStarted), int32(workerStateShutdown)) {
		// nolint: staticcheck // SA9003 not implemented yet
		if tw.workerState.Swap(int32(workerStateShutdown)) != int32(workerStateShutdown) {
			//TODO
		}
		return nil
	}

	// wait for the worker to be stopped
	<-tw.closedCh

	unprocessed := tw.unprocessedTimeouts
	cancelled := make([]Timeout, 0, len(unprocessed))
	for _, timeout := range unprocessed {
		if timeout.Cancel() {
			cancelled = append(cancelled, timeout)
		}
	}

	return cancelled
}

func (tw *WheelTimer) NewTimeout(task TimerTask, delay time.Duration) (Timeout, error) {
	pendingTimeoutsCount := tw.pendingTimeouts.Add(1)

	if tw.maxPendingTimeouts > 0 && pendingTimeoutsCount > tw.maxPendingTimeouts {
		tw.pendingTimeouts.Add(-1)
		return nil, fmt.Errorf("pending timeouts (%d) is greater than maxPendingTimeouts (%d)", pendingTimeoutsCount, tw.maxPendingTimeouts)
	}

	err := tw.Start()
	if err != nil {
		tw.pendingTimeouts.Add(-1)
		return nil, err
	}

	deadline := time.Since(tw.startTime.Load().(time.Time)) + delay
	if delay > 0 && deadline < 0 {
		deadline = math.MaxInt64
	}

	timeout := newWheelTimeout(tw, task, deadline)
	err = tw.timeouts.Put(timeout)
	if err != nil {
		tw.pendingTimeouts.Add(-1)
		return nil, err
	}

	return timeout, nil
}

func (tw *WheelTimer) State() workerState {
	return workerState(tw.workerState.Load())
}

func (tw *WheelTimer) run() {
	tw.startTime.Store(time.Now())
	tw.startTimeInitializer.Done()

	defer func() {
		// release the remaining timeouts
		tw.timeouts.Dispose()
		tw.cancelledTimeouts.Dispose()
		close(tw.closedCh)
	}()

	for tw.State() == workerStateStarted {
		deadline := tw.waitForNextTick()
		if deadline > 0 {
			idx := tw.tick & tw.mask
			tw.processCancelledTasks()
			bucket := tw.wheel[idx]
			tw.transferTimeoutsToBuckets()
			bucket.expireTimeouts(deadline)
			tw.tick++
		}
	}

	for _, bucket := range tw.wheel {
		tw.unprocessedTimeouts = bucket.clearTimeouts(tw.unprocessedTimeouts)
	}

	for {
		data, err := tw.timeouts.PollNonBlocking(0)
		if errors.Is(err, ErrEmpty) {
			break
		}
		timeout := data.(*WheelTimeout)
		if !timeout.IsCancelled() {
			tw.unprocessedTimeouts = append(tw.unprocessedTimeouts, timeout)
		}
	}
	tw.processCancelledTasks()
}

func (tw *WheelTimer) waitForNextTick() time.Duration {
	deadline := tw.tickDuration * time.Duration(tw.tick+1)
	startTime := tw.startTime.Load().(time.Time)

	for {
		currentTime := time.Since(startTime)
		sleepTimeMs := (deadline - currentTime + time.Duration(999999)).Milliseconds()

		if sleepTimeMs <= 0 {
			if currentTime.Nanoseconds() == math.MinInt64 {
				return -time.Duration(math.MaxInt64)
			} else {
				return currentTime
			}
		}

		// 微秒sleep在不同的系统上有不同的精度, 但是毫秒sleep是比较稳定的
		time.Sleep(time.Duration(sleepTimeMs))
	}
}

func (tw *WheelTimer) processCancelledTasks() {
	for {
		data, err := tw.cancelledTimeouts.PollNonBlocking(0)
		if errors.Is(err, ErrEmpty) {
			break
		}
		data.(*WheelTimeout).remove()
	}
}

func (tw *WheelTimer) transferTimeoutsToBuckets() {
	// transfer only max. 100000 timeouts per tick to prevent a thread to stale the workerThread when it just
	// adds new timeouts in a loop.
	for i := 0; i < 100000; i++ {
		data, err := tw.timeouts.PollNonBlocking(0)
		if errors.Is(err, ErrEmpty) {
			break
		}

		timeout := data.(*WheelTimeout)

		if timeout.State() == timeoutStateCancelled {
			continue
		}

		calculated := int(timeout.deadline / tw.tickDuration)
		timeout.remainingRounds = (calculated - tw.tick) / len(tw.wheel)

		var ticks int
		if calculated < tw.tick {
			ticks = tw.tick
		} else {
			ticks = int(calculated)
		}
		stopIndex := (int)(ticks & tw.mask)

		bucket := tw.wheel[stopIndex]
		bucket.addTimeout(timeout)
	}
}

func newTimerWheel(ticksPerWheel uint32) []*WheelBucket {
	ticksPerWheel = utils.FindNextPositivePowerOfTwo(ticksPerWheel)

	wheel := make([]*WheelBucket, ticksPerWheel)
	for i := range wheel {
		wheel[i] = &WheelBucket{}
	}
	return wheel
}
