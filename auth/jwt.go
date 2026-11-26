package auth

import (
	"github.com/rapide-go/rapide/auth/jwt"
	"github.com/rapide-go/rapide/routing"
)

// JWT returns a stateless Bearer authentication middleware that verifies an HS256 JWT
// into typed claims C and stores it on ctx.Ctx (retrievable via auth.User[C](c) / auth.MustUser[C](c)).
func JWT[C any](secret string) router.Middleware {
	return Bearer(func(token string) (C, error) {
		return jwt.Verify[C](token, secret)
	})
}
