package ctx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SSEWriter streams RFC-compliant Server-Sent Events directly from any Rapide handler
// (ideal for AI token streaming, live progress updates, logs, and real-time feeds).
type SSEWriter struct {
	c  *Ctx
	rc *http.ResponseController
}

// SSE upgrades the response to a Server-Sent Events stream, disables proxy buffering,
// clears the server write deadline for long-lived streams, and flushes headers immediately.
func (c *Ctx) SSE() *SSEWriter {
	rc := http.NewResponseController(c.Writer)
	_ = rc.SetWriteDeadline(time.Time{})

	c.SetHeader("Content-Type", "text/event-stream; charset=utf-8")
	c.SetHeader("Cache-Control", "no-cache")
	c.SetHeader("Connection", "keep-alive")
	c.SetHeader("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	_ = rc.Flush()

	return &SSEWriter{c: c, rc: rc}
}

// Done returns a channel closed when the client disconnects.
func (s *SSEWriter) Done() <-chan struct{} {
	return s.c.Context().Done()
}

// LastEventID returns the `Last-Event-ID` header sent by reconnecting clients.
func (s *SSEWriter) LastEventID() string {
	return s.c.Header("Last-Event-ID")
}

// Send writes an SSE event (with multi-line data safely split) and flushes immediately.
func (s *SSEWriter) Send(event, data string) error {
	return s.SendID("", event, data)
}

// SendID writes an SSE event with an explicit `id:` field for client resumption.
func (s *SSEWriter) SendID(id, event, data string) error {
	var sb strings.Builder
	if id != "" {
		fmt.Fprintf(&sb, "id: %s\n", id)
	}
	if event != "" {
		fmt.Fprintf(&sb, "event: %s\n", event)
	}
	for line := range strings.SplitSeq(data, "\n") {
		fmt.Fprintf(&sb, "data: %s\n", line)
	}
	sb.WriteByte('\n')

	if _, err := s.c.Writer.Write([]byte(sb.String())); err != nil {
		return err
	}
	return s.rc.Flush()
}

// SendJSON marshals payload to JSON, writes it as an SSE event, and flushes immediately.
func (s *SSEWriter) SendJSON(event string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.Send(event, string(raw))
}

// Comment writes an SSE comment (`: keepalive`) to keep proxies/load-balancers alive.
func (s *SSEWriter) Comment(text string) error {
	if _, err := fmt.Fprintf(s.c.Writer, ": %s\n\n", text); err != nil {
		return err
	}
	return s.rc.Flush()
}

// Retry sets the client's reconnection interval in milliseconds.
func (s *SSEWriter) Retry(ms int) error {
	if _, err := fmt.Fprintf(s.c.Writer, "retry: %d\n\n", ms); err != nil {
		return err
	}
	return s.rc.Flush()
}
