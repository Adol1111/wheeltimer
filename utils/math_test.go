package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindNextPositivePowerOfTwo(t *testing.T) {
	var tests = []struct {
		input    uint32
		expected uint32
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{0x40000000 - 1, 0x40000000},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, FindNextPositivePowerOfTwo(test.input))
	}
}
