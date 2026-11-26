package middleware

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/option"
)

// CORSOptions configures Cross-Origin Resource Sharing headers.
type CORSOptions struct {
	AllowOrigin      string
	AllowOrigins     []string
	AllowMethods     string
	AllowHeaders     string
	AllowCredentials bool
	MaxAge           int
}

func WithAllowOrigin(origin string) option.Option[CORSOptions] {
	return func(c *CORSOptions) { c.AllowOrigin = origin }
}

func WithAllowOrigins(origins ...string) option.Option[CORSOptions] {
	return func(c *CORSOptions) { c.AllowOrigins = origins }
}

func WithAllowMethods(methods string) option.Option[CORSOptions] {
	return func(c *CORSOptions) { c.AllowMethods = methods }
}

func WithAllowHeaders(headers string) option.Option[CORSOptions] {
	return func(c *CORSOptions) { c.AllowHeaders = headers }
}

func WithAllowCredentials(allow bool) option.Option[CORSOptions] {
	return func(c *CORSOptions) { c.AllowCredentials = allow }
}

func WithCORSMaxAge(seconds int) option.Option[CORSOptions] {
	return func(c *CORSOptions) { c.MaxAge = seconds }
}

// CORS attaches hardened CORS headers and short-circuits OPTIONS preflight requests.
// Refuses to emit Access-Control-Allow-Credentials alongside a wildcard "*" origin.
func CORS(opts ...option.Option[CORSOptions]) router.Middleware {
	cfg := CORSOptions{
		AllowOrigin:  "*",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowHeaders: "Accept, Authorization, Content-Type, X-Requested-With",
		MaxAge:       600,
	}
	option.Apply(&cfg, opts...)

	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			origin := c.Header("Origin")
			allowedOrigin := ""
			if len(cfg.AllowOrigins) > 0 {
				if origin != "" && slices.Contains(cfg.AllowOrigins, origin) {
					allowedOrigin = origin
					c.Writer.Header().Add("Vary", "Origin")
				}
			} else if cfg.AllowOrigin != "" {
				allowedOrigin = cfg.AllowOrigin
				if allowedOrigin != "*" {
					c.Writer.Header().Add("Vary", "Origin")
				}
			}

			if allowedOrigin != "" {
				c.SetHeader("Access-Control-Allow-Origin", allowedOrigin)
				c.SetHeader("Access-Control-Allow-Methods", cfg.AllowMethods)
				c.SetHeader("Access-Control-Allow-Headers", cfg.AllowHeaders)
				if cfg.AllowCredentials && allowedOrigin != "*" {
					c.SetHeader("Access-Control-Allow-Credentials", "true")
				}
				if cfg.MaxAge > 0 {
					c.SetHeader("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
				}
			}

			if c.Request.Method == http.MethodOptions {
				return c.NoContent()
			}
			return next(c)
		}
	}
}
