package setup

import "time"

// Clock is an interface that allows us to mock time in tests.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

// RealClock implements Clock using the standard time package.
type RealClock struct{}

func (c *RealClock) Now() time.Time {
	return time.Now()
}

func (c *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}
