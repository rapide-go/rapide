package middleware

import (
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/option"
)

// SecurityOptions configures HTTP security response headers.
type SecurityOptions struct {
	FrameOptions      string
	ReferrerPolicy    string
	PermissionsPolicy string
	ContentSecurity   string
	HSTSMaxAge        string
}

func WithFrameOptions(val string) option.Option[SecurityOptions] {
	return func(o *SecurityOptions) { o.FrameOptions = val }
}

func WithCSP(policy string) option.Option[SecurityOptions] {
	return func(o *SecurityOptions) { o.ContentSecurity = policy }
}

func WithReferrerPolicy(val string) option.Option[SecurityOptions] {
	return func(o *SecurityOptions) { o.ReferrerPolicy = val }
}

// SecurityHeaders attaches production-hardened HTTP security headers on every response.
// Automatically adds HSTS when served over TLS or behind an HTTPS reverse proxy.
func SecurityHeaders(opts ...option.Option[SecurityOptions]) router.Middleware {
	cfg := SecurityOptions{
		FrameOptions:      "DENY",
		ReferrerPolicy:    "strict-origin-when-cross-origin",
		PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
		HSTSMaxAge:        "max-age=31536000; includeSubDomains",
	}
	option.Apply(&cfg, opts...)

	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			c.SetHeader("X-Content-Type-Options", "nosniff")
			if cfg.FrameOptions != "" {
				c.SetHeader("X-Frame-Options", cfg.FrameOptions)
			}
			if cfg.ReferrerPolicy != "" {
				c.SetHeader("Referrer-Policy", cfg.ReferrerPolicy)
			}
			if cfg.PermissionsPolicy != "" {
				c.SetHeader("Permissions-Policy", cfg.PermissionsPolicy)
			}
			if cfg.ContentSecurity != "" {
				c.SetHeader("Content-Security-Policy", cfg.ContentSecurity)
			}
			if cfg.HSTSMaxAge != "" && (c.Request.TLS != nil || c.Header("X-Forwarded-Proto") == "https") {
				c.SetHeader("Strict-Transport-Security", cfg.HSTSMaxAge)
			}
			return next(c)
		}
	}
}
