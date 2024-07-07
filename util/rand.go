package util

import "math/rand"

func RandInt(min int, max int) int {
	if max <= min {
		return min
	}

	return min + rand.Intn(max-min)
}
