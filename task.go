package wheeltimer

import "time"

type TimerTask interface {
	// Run is Executed after the delay specified with newTimeout.
	Run(timeout Timeout) error
}

type Timer interface {
	// NewTimeout is Schedules the specified TimerTask for one-time execution after the specified delay.
	NewTimeout(task TimerTask, delay time.Duration) (Timeout, error)

	// Stop is Releases all resources acquired by this Timer and cancels all
	// tasks which were scheduled but not executed yet.
	Stop() []Timeout
}

type Timeout interface {
	// Timer is Returns the Timer that created this handle.
	Timer() Timer

	// Task is Returns the TimerTask which is associated with this handle.
	Task() TimerTask

	// IsExpired is Returns true if and only if the TimerTask associated with this handle has been expired.
	IsExpired() bool

	// IsCancelled is Returns true if and only if the TimerTask associated with this handle has been cancelled.
	IsCancelled() bool

	// Cancel is Attempts to cancel the TimerTask associated with this handle.
	// If the task has been executed or cancelled already, it will return with no side effect.
	Cancel() bool
}

// TimerTaskFunc is a function type that implements TimerTask.
type TimerTaskFunc func(Timeout) error

func (f TimerTaskFunc) Run(timeout Timeout) error {
	return f(timeout)
}

type DataTimerTask[T any] struct {
	data T
	f    func(Timeout, T) error
}

func (f DataTimerTask[T]) Run(timeout Timeout) error {
	return f.f(timeout, f.data)
}

// NewDataTimerTask is a factory function that creates a new DataTimerTask.
func NewDataTimerTask[T any](data T, f func(Timeout, T) error) TimerTask {
	return DataTimerTask[T]{
		data: data,
		f:    f,
	}
}
