package middleware

import (
	"time"

	"github.com/rapide-go/rapide/http/ratelimit"
	router "github.com/rapide-go/rapide/routing"
)

// Throttle enforces a fixed-window rate limit per client IP,
// backed by Rapide's http/ratelimit package.
func Throttle(limit int, window time.Duration) router.Middleware {
	return ratelimit.New().EnforceLimit("throttle", ratelimit.Limit{
		MaxAttempts: limit,
		Decay:       window,
	})
}

// ThrottleNamed enforces a named rate limiter registered via ratelimit.For(name, fn)
// .
func ThrottleNamed(name string) router.Middleware {
	return ratelimit.Middleware(name)
}
