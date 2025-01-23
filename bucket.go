package wheeltimer

import (
	"fmt"
	"time"

	"github.com/adol1111/wheeltimer/assert"
)

type WheelBucket struct {
	head *WheelTimeout
	tail *WheelTimeout
}

func (b *WheelBucket) addTimeout(timeout *WheelTimeout) {
	assert.True(timeout.bucket == nil, "timeout.bucket should be nil")

	timeout.bucket = b
	if b.head == nil {
		b.head = timeout
		b.tail = timeout
	} else {
		b.tail.next = timeout
		timeout.prev = b.tail
		b.tail = timeout
	}
}

func (b *WheelBucket) remove(timeout *WheelTimeout) *WheelTimeout {
	next := timeout.next
	if timeout.prev != nil {
		timeout.prev.next = next
	}
	if timeout.next != nil {
		timeout.next.prev = timeout.prev
	}
	if b.head == timeout {
		if b.tail == timeout {
			b.head = nil
			b.tail = nil
		} else {
			b.head = next
		}
	} else if b.tail == timeout {
		b.tail = timeout.prev
	}

	timeout.prev = nil
	timeout.next = nil
	timeout.bucket = nil
	timeout.timer.pendingTimeouts.Add(-1)
	return next
}

func (b *WheelBucket) expireTimeouts(deadline time.Duration) {
	timeout := b.head

	for timeout != nil {
		next := timeout.next
		if timeout.remainingRounds <= 0 {
			next = b.remove(timeout)
			if timeout.deadline <= deadline {
				timeout.Expired()
			} else {
				// The timeout was placed into a wrong slot. This should never happen.
				err := fmt.Errorf("timeout.deadline(%d) > deadline(%d)", timeout.deadline, deadline)
				panic(err)
			}
		} else if timeout.IsCancelled() {
			next = b.remove(timeout)
		} else {
			timeout.remainingRounds--
		}
		timeout = next
	}
}

func (b *WheelBucket) clearTimeouts(unprocessedTimeouts []*WheelTimeout) []*WheelTimeout {
	for {
		timeout := b.pollTimeout()
		if timeout == nil {
			return unprocessedTimeouts
		}
		if timeout.IsCancelled() || timeout.IsExpired() {
			continue
		}
		unprocessedTimeouts = append(unprocessedTimeouts, timeout)
	}
}

func (b *WheelBucket) pollTimeout() *WheelTimeout {
	head := b.head
	if head == nil {
		return nil
	}
	next := head.next
	if next == nil {
		b.head = nil
		b.tail = nil
	} else {
		b.head = next
		next.prev = nil
	}

	head.prev = nil
	head.next = nil
	head.bucket = nil
	return head
}
