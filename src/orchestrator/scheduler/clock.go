package scheduler

import "time"

// Timer is a one-shot timer that can be stopped.
type Timer interface {
	Stop() bool
}

// Clock abstracts time operations for testability.
type Clock interface {
	Now() time.Time
	// NewTimer fires fn once after d. Returns a Timer that can stop it.
	NewTimer(d time.Duration, fn func()) Timer
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) NewTimer(d time.Duration, fn func()) Timer {
	t := time.AfterFunc(d, fn)
	return t
}
