package clean

import "time"

type Clock func() time.Time

func Age(now Clock, born time.Time) time.Duration { return now().Sub(born) }

func Real() time.Time { return time.Now() }
