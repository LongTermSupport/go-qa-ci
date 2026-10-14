package red

import "time"

// Production code may read the clock: the rule is about tests.
func Stamp() time.Time { return time.Now() }
