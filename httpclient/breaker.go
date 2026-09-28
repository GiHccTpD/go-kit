package httpclient

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen means the downstream service is temporarily blocked by the breaker.
var ErrCircuitOpen = errors.New("HTTP circuit open")

type breakerState uint8

const (
	stateClosed breakerState = iota
	stateOpen
	stateHalfOpen
)

type callOutcome uint8

const (
	outcomeSuccess callOutcome = iota
	outcomeFailure
	outcomeNeutral
)

type breaker struct {
	mu           sync.Mutex
	state        breakerState
	failures     int
	threshold    int
	openDuration time.Duration
	openUntil    time.Time
}

// newBreaker creates a breaker whose state is private to one SDK instance.
func newBreaker(threshold int, openDuration time.Duration) *breaker {
	return &breaker{threshold: threshold, openDuration: openDuration}
}

// allow admits normal calls or exactly one probe after the open interval.
func (b *breaker) allow(now time.Time) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case stateOpen:
		if now.Before(b.openUntil) {
			return false, ErrCircuitOpen
		}
		b.state = stateHalfOpen
		return true, nil
	case stateHalfOpen:
		return false, ErrCircuitOpen
	default:
		return false, nil
	}
}

// record counts a full logical call once and resolves a half-open probe.
func (b *breaker) record(probe bool, outcome callOutcome) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if probe {
		if outcome == outcomeSuccess {
			b.state = stateClosed
			b.failures = 0
		} else {
			b.state = stateOpen
			b.openUntil = time.Now().Add(b.openDuration)
		}
		return
	}
	if b.state != stateClosed || outcome == outcomeNeutral {
		return
	}
	if outcome == outcomeSuccess {
		b.failures = 0
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.state = stateOpen
		b.openUntil = time.Now().Add(b.openDuration)
	}
}
