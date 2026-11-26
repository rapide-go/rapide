package inertia

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	igo "github.com/rapide-go/inertia"
	broadcast "github.com/rapide-go/rapide/broadcasting"
	"github.com/rapide-go/rapide/events"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

const (
	liveChannelPrefix  = "inertia.live:"
	maxLiveChannels    = 16
	maxChannelNameLen  = 64
	liveKeepaliveEvery = 25 * time.Second
)

// MaxLiveConnections caps simultaneous `/_inertia/live` SSE streams to prevent goroutine/FD exhaustion.
var MaxLiveConnections int64 = 256

var (
	activeLiveConns atomic.Int64

	liveAuthMu  sync.RWMutex
	liveAuthFns = map[string]LiveAuthorizer{}
)

// LiveAuthorizer decides whether the current HTTP request may subscribe to channel.
type LiveAuthorizer func(c *ctx.Ctx, channel string) bool

// AuthorizeLive registers a channel authorization callback.
// Pattern may be an exact channel name (e.g. "private.orders") or a prefix ending in "*" (e.g. "private.user.*").
// Any channel prefixed with "private." or "presence." is denied by default unless explicitly authorized.
func AuthorizeLive(pattern string, fn LiveAuthorizer) {
	if pattern == "" || fn == nil {
		return
	}
	liveAuthMu.Lock()
	liveAuthFns[pattern] = fn
	liveAuthMu.Unlock()
}

func isChannelAuthorized(c *ctx.Ctx, ch string) bool {
	liveAuthMu.RLock()
	defer liveAuthMu.RUnlock()

	if fn, ok := liveAuthFns[ch]; ok {
		return fn(c, ch)
	}
	for pat, fn := range liveAuthFns {
		if prefix, ok := strings.CutSuffix(pat, "*"); ok && strings.HasPrefix(ch, prefix) {
			return fn(c, ch)
		}
	}
	// Fail closed for private/presence channels when no authorizer matches.
	if strings.HasPrefix(ch, "private.") || strings.HasPrefix(ch, "presence.") {
		return false
	}
	return true
}

type livePing struct {
	Channel string `json:"channel"`
	Event   string `json:"event"`
}

// Live wraps a value as an Inertia v3 Live prop that automatically reloads in connected browsers
// whenever NotifyLive(channel, event) or LiveOnEvent[E](channel, event) fires.
func Live[T any](value T, channel string, eventNames ...string) *igo.Prop {
	return igo.Live(value, igo.NewLiveListener(igo.PublicChannel(channel), eventNames...))
}

// NotifyLive broadcasts a lightweight SSE invalidation ping to all connected browsers
// listening to channel, triggering an automatic targeted Inertia partial reload.
func NotifyLive(channel, event string) error {
	if !isSafeChannelName(channel) || event == "" || strings.ContainsAny(event, "\r\n") {
		return nil
	}
	return broadcast.Publish(liveChannelPrefix+channel, "live", livePing{
		Channel: channel,
		Event:   event,
	})
}

// LiveOnEvent bridges a typed Rapide domain event E directly to Live Inertia prop invalidation.
// Whenever `events.Dispatch(E{...})` is called anywhere in the app, connected browsers
// subscribed to `channel` automatically reload only the props listening to `eventName`.
func LiveOnEvent[E any](channel, eventName string) {
	events.Listen(func(_ E) {
		_ = NotifyLive(channel, eventName)
	})
}

// MountLive mounts the multiplexed SSE endpoint (default `/_inertia/live`) backed by Rapide's
// in-process broadcasting Hub. Browsers connect via `?channels=dashboard,blog` (max 16 channels).
func MountLive(r *router.Router, path ...string) {
	p := "/_inertia/live"
	if len(path) > 0 && path[0] != "" {
		p = path[0]
	}
	r.Get(p, LiveHandlerOn(broadcast.Default)).SetName("inertia.live")
}

// LiveHandlerOn returns a hardened multiplexed SSE handler subscribing to the requested `?channels=...`
// on hub with connection capping, channel authorization, and keepalive heartbeats.
func LiveHandlerOn(hub *broadcast.Hub) router.Handler {
	return func(c *ctx.Ctx) error {
		channels := parseLiveChannels(c.Query("channels"))
		if len(channels) == 0 {
			return ctx.Abort(400, "at least one valid channel is required")
		}
		for _, chName := range channels {
			if !isChannelAuthorized(c, chName) {
				return ctx.Forbidden("unauthorized live channel")
			}
		}

		if MaxLiveConnections > 0 && activeLiveConns.Add(1) > MaxLiveConnections {
			activeLiveConns.Add(-1)
			return ctx.Abort(503, "too many active live connections")
		}
		defer activeLiveConns.Add(-1)

		c.SetHeader("Cache-Control", "no-cache, no-store")
		c.SetHeader("X-Accel-Buffering", "no")

		sse := c.SSE()
		merged := make(chan broadcast.Message, 32)
		var (
			wg     sync.WaitGroup
			unsubs = make([]func(), 0, len(channels))
		)

		for _, chName := range channels {
			sub, unsub := hub.Subscribe(liveChannelPrefix + chName)
			unsubs = append(unsubs, unsub)
			wg.Add(1)
			go func(src <-chan broadcast.Message) {
				defer wg.Done()
				for {
					select {
					case <-sse.Done():
						return
					case msg, ok := <-src:
						if !ok {
							return
						}
						select {
						case merged <- msg:
						case <-sse.Done():
							return
						default:
						}
					}
				}
			}(sub)
		}

		defer func() {
			for _, u := range unsubs {
				u()
			}
			wg.Wait()
		}()

		ticker := time.NewTicker(liveKeepaliveEvery)
		defer ticker.Stop()

		for {
			select {
			case <-sse.Done():
				return nil
			case <-ticker.C:
				if err := sse.Send("ping", "{}"); err != nil {
					return nil
				}
			case msg := <-merged:
				if err := sse.Send(msg.Event, string(msg.Data)); err != nil {
					return nil
				}
			}
		}
	}
}

func parseLiveChannels(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for part := range strings.SplitSeq(raw, ",") {
		ch := strings.TrimSpace(part)
		if !isSafeChannelName(ch) || seen[ch] {
			continue
		}
		seen[ch] = true
		out = append(out, ch)
		if len(out) >= maxLiveChannels {
			break
		}
	}
	return out
}

func isSafeChannelName(ch string) bool {
	if len(ch) == 0 || len(ch) > maxChannelNameLen {
		return false
	}
	for i := 0; i < len(ch); i++ {
		b := ch[i]
		ok := (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
			b == '.' || b == '_' || b == '-' || b == ':'
		if !ok {
			return false
		}
	}
	return true
}
