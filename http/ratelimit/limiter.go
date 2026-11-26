package ratelimit

import (
	"sync"
	"time"
)

// Limit describes a rate-limit window.
type Limit struct {
	MaxAttempts int
	Decay       time.Duration
	Key         string
	Unlimited   bool
}

// PerSecond allows max attempts per second.
func PerSecond(max int) Limit { return Limit{MaxAttempts: max, Decay: time.Second} }

// PerMinute allows max attempts per minute.
func PerMinute(max int) Limit { return Limit{MaxAttempts: max, Decay: time.Minute} }

// PerMinutes allows max attempts per decayMinutes minutes.
func PerMinutes(decayMinutes, max int) Limit {
	return Limit{MaxAttempts: max, Decay: time.Duration(decayMinutes) * time.Minute}
}

// PerHour allows max attempts per hour.
func PerHour(max int) Limit { return Limit{MaxAttempts: max, Decay: time.Hour} }

// PerDay allows max attempts per 24 hours.
func PerDay(max int) Limit { return Limit{MaxAttempts: max, Decay: 24 * time.Hour} }

// None disables rate limiting for the request.
func None() Limit { return Limit{Unlimited: true} }

// By partitions the rate-limit bucket by key (e.g. user ID, email, or IP).
func (l Limit) By(key string) Limit {
	l.Key = key
	return l
}

type bucket struct {
	hits      int
	expiresAt time.Time
}

// Result reports the outcome of a rate-limit attempt.
type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Limit      int
}

// Limiter is a thread-safe, in-memory named rate limiter.
// It automatically prunes expired buckets to stay memory-bounded under high-cardinality traffic.
type Limiter struct {
	mu        sync.Mutex
	named     map[string]LimiterFunc
	buckets   map[string]*bucket
	nowFunc   func() time.Time
	lastSweep time.Time
}

// New creates a standalone Limiter instance.
func New() *Limiter {
	now := time.Now
	return &Limiter{
		named:     make(map[string]LimiterFunc),
		buckets:   make(map[string]*bucket),
		nowFunc:   now,
		lastSweep: now(),
	}
}

// Default is the global rate limiter used by package-level facade helpers.
var Default = New()

// For registers a named rate limiter (e.g. ratelimit.For("login",...)).
func For(name string, fn LimiterFunc) { Default.For(name, fn) }

// For registers a named rate limiter on this Limiter instance.
func (l *Limiter) For(name string, fn LimiterFunc) {
	l.mu.Lock()
	l.named[name] = fn
	l.mu.Unlock()
}

// Hit records one attempt against key for lim and returns whether it is allowed.
func (l *Limiter) Hit(key string, lim Limit) Result {
	if lim.Unlimited || lim.MaxAttempts <= 0 {
		return Result{Allowed: true, Remaining: lim.MaxAttempts, Limit: lim.MaxAttempts}
	}
	if lim.Decay <= 0 {
		lim.Decay = time.Minute
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFunc()
	if now.Sub(l.lastSweep) > lim.Decay && len(l.buckets) > 512 {
		for k, bk := range l.buckets {
			if !now.Before(bk.expiresAt) {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	b, ok := l.buckets[key]
	if !ok || !now.Before(b.expiresAt) {
		b = &bucket{hits: 0, expiresAt: now.Add(lim.Decay)}
		l.buckets[key] = b
	}

	if b.hits >= lim.MaxAttempts {
		return Result{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: b.expiresAt.Sub(now),
			Limit:      lim.MaxAttempts,
		}
	}

	b.hits++
	return Result{
		Allowed:   true,
		Remaining: lim.MaxAttempts - b.hits,
		Limit:     lim.MaxAttempts,
	}
}

// Clear resets the attempt counter for key.
func (l *Limiter) Clear(key string) {
	l.mu.Lock()
	delete(l.buckets, key)
	l.mu.Unlock()
}

// Remaining returns how many attempts remain for key under lim without incrementing.
func (l *Limiter) Remaining(key string, lim Limit) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || !l.nowFunc().Before(b.expiresAt) {
		return lim.MaxAttempts
	}
	return max(0, lim.MaxAttempts-b.hits)
}
