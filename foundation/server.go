package foundation

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rapide-go/rapide/support/option"
)

// ServerOptions configures hardened production HTTP server timeouts and limits.
type ServerOptions struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
}

// DefaultServerOptions returns production-safe HTTP server timeouts.
func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ShutdownTimeout:   15 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
}

// WithReadTimeout sets the HTTP server's ReadTimeout.
func WithReadTimeout(d time.Duration) option.Option[ServerOptions] {
	return func(o *ServerOptions) { o.ReadTimeout = d }
}

// WithWriteTimeout sets the HTTP server's WriteTimeout.
func WithWriteTimeout(d time.Duration) option.Option[ServerOptions] {
	return func(o *ServerOptions) { o.WriteTimeout = d }
}

// WithIdleTimeout sets the HTTP server's IdleTimeout.
func WithIdleTimeout(d time.Duration) option.Option[ServerOptions] {
	return func(o *ServerOptions) { o.IdleTimeout = d }
}

// WithShutdownTimeout sets the graceful shutdown drain timeout.
func WithShutdownTimeout(d time.Duration) option.Option[ServerOptions] {
	return func(o *ServerOptions) { o.ShutdownTimeout = d }
}

// Serve boots the application and starts the HTTP server with DefaultServerOptions
// plus any optional generic option.Option[ServerOptions].
func (a *App) Serve(addr string, handler http.Handler, opts ...option.Option[ServerOptions]) error {
	cfg := DefaultServerOptions()
	option.Apply(&cfg, opts...)
	return a.ServeWithOptions(addr, handler, cfg)
}

// ServeWithOptions boots the application, starts the HTTP server with custom timeouts,
// and gracefully shuts down on SIGINT/SIGTERM.
func (a *App) ServeWithOptions(addr string, handler http.Handler, opts ServerOptions) error {
	if err := a.Boot(); err != nil {
		return err
	}

	def := DefaultServerOptions()
	if opts.ReadHeaderTimeout <= 0 {
		opts.ReadHeaderTimeout = def.ReadHeaderTimeout
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = def.ReadTimeout
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = def.WriteTimeout
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = def.IdleTimeout
	}
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = def.ShutdownTimeout
	}
	if opts.MaxHeaderBytes <= 0 {
		opts.MaxHeaderBytes = def.MaxHeaderBytes
	}

	a.httpServer = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: opts.ReadHeaderTimeout,
		ReadTimeout:       opts.ReadTimeout,
		WriteTimeout:      opts.WriteTimeout,
		IdleTimeout:       opts.IdleTimeout,
		MaxHeaderBytes:    opts.MaxHeaderBytes,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		ctx, cancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout)
		defer cancel()
		return a.Shutdown(ctx)
	}
}
