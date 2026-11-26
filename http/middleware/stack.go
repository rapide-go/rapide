package middleware

import (
	"github.com/rapide-go/rapide/lang"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/session"
)

// WebDefaults returns Rapide's hardened default global middleware stack
// (RequestID, Recover, SecurityHeaders, Logger, CORS, Locale, Session) —
// mirroring Rapide's default web middleware group.
func WebDefaults(extra ...router.Middleware) []router.Middleware {
	stack := []router.Middleware{
		RequestID(),
		Recover(),
		SecurityHeaders(),
		Logger(),
		CORS(),
		lang.Middleware(""),
		session.MiddlewareFromConfig(),
	}
	return append(stack, extra...)
}
