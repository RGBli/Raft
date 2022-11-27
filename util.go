package main

import (
	"math/rand"
	"time"
)

// randDuration returns random time duration
func randDuration(minDuration time.Duration, maxDuration time.Duration) time.Duration {
	extra := time.Duration(rand.Int63()) % (maxDuration - minDuration)
	return minDuration + extra
}
