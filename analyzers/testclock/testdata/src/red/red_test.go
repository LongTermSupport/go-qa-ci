package red

import (
	"testing"
	"time"
	clock "time"
)

func TestWallClock(t *testing.T) {
	start := time.Now()   // want `time\.Now reads the wall clock in a test`
	_ = time.Since(start) // want `time\.Since reads the wall clock in a test`
	_ = time.Until(start) // want `time\.Until reads the wall clock in a test`
	_ = clock.Now()       // want `time\.Now reads the wall clock in a test`
}

var now = time.Now // want `time\.Now reads the wall clock in a test`

func helper() time.Time {
	return time.Now() // want `time\.Now reads the wall clock in a test`
}
