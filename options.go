package wheeltimer

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

const (
	// DefaultMaxPendingTimeouts is the default maximum number of pending timeouts.
	DefaultMaxPendingTimeouts = 512
)

type option struct {
	maxPendingTimeouts int64
	executor           Executor
	panicHandler       PanicHandler
	logger             *slog.Logger
}

type WheelTimerOption func(*option)

func WithExecutor(executor Executor) WheelTimerOption {
	return func(o *option) {
		o.executor = executor
	}
}

func WithPanicHandler(handler PanicHandler) WheelTimerOption {
	return func(o *option) {
		o.panicHandler = handler
	}
}

func WithLogger(logger *slog.Logger) WheelTimerOption {
	return func(o *option) {
		o.logger = logger
	}
}

func WithMaxPendingTimeouts(maxPendingTimeouts int64) WheelTimerOption {
	return func(o *option) {
		o.maxPendingTimeouts = maxPendingTimeouts
	}
}

type Executor interface {
	Execute(task func())
}

type PanicHandler func(interface{})

type defaultExecutor struct{}

func (d *defaultExecutor) Execute(task func()) {
	go task()
}

func newDefaultOption() *option {
	logger := slog.Default()
	return &option{
		executor: &defaultExecutor{},
		panicHandler: func(p interface{}) {
			logger.Error(fmt.Sprintf("[wheeltimer] panic: %v\n%s\n", p, debug.Stack()))
		},
		logger:             logger,
		maxPendingTimeouts: DefaultMaxPendingTimeouts,
	}
}
