package wheeltimer

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type WheelTimeout struct {
	timer           *WheelTimer
	task            TimerTask
	state           atomic.Int32
	deadline        time.Duration
	remainingRounds int

	next *WheelTimeout
	prev *WheelTimeout

	bucket *WheelBucket
}

func newWheelTimeout(timer *WheelTimer, task TimerTask, deadline time.Duration) *WheelTimeout {
	return &WheelTimeout{
		timer:    timer,
		task:     task,
		deadline: deadline,
	}
}

func (timeout *WheelTimeout) Timer() Timer {
	return timeout.timer
}

func (timeout *WheelTimeout) Task() TimerTask {
	return timeout.task
}

func (timeout *WheelTimeout) IsExpired() bool {
	return timeout.State() == timeoutStateExpired
}

func (timeout *WheelTimeout) IsCancelled() bool {
	return timeout.State() == timeoutStateCancelled
}

func (timeout *WheelTimeout) State() timeoutState {
	return timeoutState(timeout.state.Load())
}

func (timeout *WheelTimeout) Cancel() bool {
	if !timeout.state.CompareAndSwap(int32(timeoutStateInit), int32(timeoutStateCancelled)) {
		return false
	}
	// this error does not need to be handled, because if the write fails, it means that the wheeltimer has stopped,
	// and no one is consuming cancelledTimeouts at this time, so it needs to return true to let the goroutine that calls stop handle it.
	// else if the wheeltimer has not stopped, always write success.
	_ = timeout.timer.cancelledTimeouts.Put(timeout)
	return true
}

func (timeout *WheelTimeout) remove() {
	if timeout.bucket != nil {
		timeout.bucket.remove(timeout)
	} else {
		timeout.timer.pendingTimeouts.Add(-1)
	}
}

func (timeout *WheelTimeout) Expired() {
	if !timeout.state.CompareAndSwap(int32(timeoutStateInit), int32(timeoutStateExpired)) {
		return
	}

	timeout.timer.executor.Execute(timeout.run)
}

func (timeout *WheelTimeout) run() {
	defer func() {
		if r := recover(); r != nil {
			timeout.timer.panicHandler(r)
		}
	}()
	err := timeout.task.Run(timeout)
	if err != nil {
		timeout.timer.logger.Warn("[wheeltimer] task run error", "error", err)
	}
}

func (timeout *WheelTimeout) String() string {
	startTime, ok := timeout.timer.startTime.Load().(time.Time)
	if !ok {
		startTime = time.Time{}
	}
	remaining := timeout.deadline - time.Since(startTime)
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("(deadline: %d", remaining))
	if remaining > 0 {
		buf.WriteString(fmt.Sprintf("%d ns later", remaining))
	} else if remaining < 0 {
		buf.WriteString(fmt.Sprintf("%d ns ago", -remaining))
	} else {
		buf.WriteString("now")
	}

	if timeout.IsCancelled() {
		buf.WriteString(", cancelled")
	}

	buf.WriteString(fmt.Sprintf(", task: %v)", timeout.task))
	return buf.String()
}
