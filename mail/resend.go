package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ResendDriver delivers emails via the Resend HTTP API (https://api.resend.com/emails)
// using pure Go stdlib net/http (zero third-party SDK dependencies).
type ResendDriver struct {
	APIKey   string
	From     string
	Endpoint string
	Client   *http.Client
}

// NewResend creates a Resend API mailer driver.
func NewResend(apiKey, defaultFrom string) *ResendDriver {
	return &ResendDriver{
		APIKey:   apiKey,
		From:     defaultFrom,
		Endpoint: "https://api.resend.com/emails",
		Client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (r *ResendDriver) Send(ctx context.Context, msg Message) error {
	from := msg.From
	if from == "" {
		from = r.From
	}
	payload := map[string]any{
		"from":    from,
		"to":      msg.To,
		"subject": msg.Subject,
	}
	if len(msg.CC) > 0 {
		payload["cc"] = msg.CC
	}
	if len(msg.BCC) > 0 {
		payload["bcc"] = msg.BCC
	}
	if msg.ReplyTo != "" {
		payload["reply_to"] = msg.ReplyTo
	}
	if msg.Text != "" {
		payload["text"] = msg.Text
	}
	if msg.HTML != "" {
		payload["html"] = msg.HTML
	}
	if len(msg.Attachments) > 0 {
		atts := make([]map[string]string, len(msg.Attachments))
		for i, a := range msg.Attachments {
			atts[i] = map[string]string{
				"filename": a.Filename,
				"content":  base64.StdEncoding.EncodeToString(a.Data),
			}
		}
		payload["attachments"] = atts
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = "https://api.resend.com/emails"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("mail.resend: API returned status %d", resp.StatusCode)
	}
	return nil
}

// MemoryDriver records sent messages in memory (ideal for faking Mail() in tests).
type MemoryDriver struct {
	mu   sync.RWMutex
	sent []Message
}

func NewMemory() *MemoryDriver { return &MemoryDriver{} }

func (m *MemoryDriver) Send(_ context.Context, msg Message) error {
	m.mu.Lock()
	m.sent = append(m.sent, msg)
	m.mu.Unlock()
	return nil
}

// Sent returns a snapshot of all recorded messages.
func (m *MemoryDriver) Sent() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]Message(nil), m.sent...)
}
