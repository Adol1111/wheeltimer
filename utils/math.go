package utils

import (
	"math/bits"
)

func NumberOfLeadingZeros(n uint32) int {
	if n == 0 {
		return 32
	}
	return bits.LeadingZeros32(n)
}

func FindNextPositivePowerOfTwo(value uint32) uint32 {
	if value <= 0 || value >= 0x40000000 {
		return 0
	}
	return 1 << (32 - NumberOfLeadingZeros(value-1))
}
