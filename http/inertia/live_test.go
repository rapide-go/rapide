package inertia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	broadcast "github.com/rapide-go/rapide/broadcasting"
	"github.com/rapide-go/rapide/events"
	"github.com/rapide-go/rapide/http/ctx"
)

type postCreatedEvent struct {
	ID int
}

func TestLiveOnEventBroadcastsInvalidation(t *testing.T) {
	ch, unsub := broadcast.Default.Subscribe(liveChannelPrefix + "dashboard")
	defer unsub()

	LiveOnEvent[postCreatedEvent]("dashboard", "post.created")
	events.Dispatch(postCreatedEvent{ID: 7})

	select {
	case msg := <-ch:
		if msg.Event != "live" {
			t.Fatalf("expected event 'live', got %q", msg.Event)
		}
		var ping livePing
		if err := json.Unmarshal(msg.Data, &ping); err != nil {
			t.Fatalf("unmarshal ping: %v", err)
		}
		if ping.Channel != "dashboard" || ping.Event != "post.created" {
			t.Fatalf("unexpected ping: %+v", ping)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live invalidation broadcast")
	}
}

func TestParseLiveChannelsCapsAndDeduplicates(t *testing.T) {
	got := parseLiveChannels("blog, dashboard, blog, bad\r\nch, ../evil, stats")
	if len(got) != 3 || got[0] != "blog" || got[1] != "dashboard" || got[2] != "stats" {
		t.Fatalf("unexpected parsed channels: %#v", got)
	}
	if isSafeChannelName(strings.Repeat("a", maxChannelNameLen+1)) {
		t.Fatal("expected over-length channel name to be rejected")
	}
}

func TestLiveChannelAuthorizationFailClosed(t *testing.T) {
	handler := LiveHandlerOn(broadcast.NewHub())

	// Private channel without authorizer -> 403 Forbidden
	c := ctx.New(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/_inertia/live?channels=private.secret", nil))
	err := handler(c)
	httpErr, ok := err.(*ctx.HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized private channel, got %v", err)
	}

	// Register wildcard authorizer that checks header
	AuthorizeLive("private.team.*", func(c *ctx.Ctx, _ string) bool {
		return c.Header("X-Allow") == "1"
	})

	cDenied := ctx.New(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/_inertia/live?channels=private.team.42", nil))
	if err := handler(cDenied); err == nil {
		t.Fatal("expected 403 when authorizer returns false")
	}

	// Verify connection saturation returns 503
	prevMax := MaxLiveConnections
	MaxLiveConnections = 1
	activeLiveConns.Store(1)
	defer func() {
		activeLiveConns.Store(0)
		MaxLiveConnections = prevMax
	}()

	reqAllowed := httptest.NewRequest(http.MethodGet, "/_inertia/live?channels=private.team.42", nil)
	reqAllowed.Header.Set("X-Allow", "1")
	cSat := ctx.New(httptest.NewRecorder(), reqAllowed)
	err = handler(cSat)
	httpErr, ok = err.(*ctx.HTTPError)
	if !ok || httpErr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when saturated, got %v", err)
	}
}
