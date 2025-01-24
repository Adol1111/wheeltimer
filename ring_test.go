package wheeltimer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingBuffer_PollEmpty(t *testing.T) {
	ring := NewRingBuffer(2)
	_, err := ring.PollNonBlocking(0)
	assert.ErrorIs(t, err, ErrEmpty)

	for i := 0; i < 2; i++ {
		_ = ring.Put(i)
		_, _ = ring.Poll(0)
	}
	_, err = ring.PollNonBlocking(0)
	assert.ErrorIs(t, err, ErrEmpty)
}
