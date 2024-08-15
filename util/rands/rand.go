package rands

import (
	"math/rand/v2"
)

func RandInt(min, max int) int {
	if max <= min {
		return min
	}

	return min + rand.IntN(max-min)
}
