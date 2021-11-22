package main

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// randDuration returns random time duration
func randDuration(minDuration time.Duration) time.Duration {
	extra := time.Duration(rand.Int63()) % minDuration
	return minDuration + extra
}