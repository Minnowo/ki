package storage

import (
	"sync/atomic"
	"time"
)

// activityClock records the last time a session was used.
// It is safe to use without holding the session lock, so the sweepers can find idle sessions without waiting on busy ones.
type activityClock struct {
	unixNano atomic.Int64
}

// Touch sets the last activity to now.
func (a *activityClock) Touch() {
	a.Set(time.Now())
}

func (a *activityClock) Set(t time.Time) {
	a.unixNano.Store(t.UnixNano())
}

func (a *activityClock) Get() time.Time {
	return time.Unix(0, a.unixNano.Load())
}

// IdleFor returns true if the last activity was more than timeout before now.
func (a *activityClock) IdleFor(now time.Time, timeout time.Duration) bool {
	return now.Sub(a.Get()) > timeout
}
