package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a fluent, retry-capable HTTP client.
type Client struct {
	http    *http.Client
	headers map[string]string
	retries int
	backoff time.Duration
}

// New creates a Client with sensible production defaults (15s timeout).
func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second},
		headers: make(map[string]string),
		backoff: 250 * time.Millisecond,
	}
}

func (c *Client) WithTimeout(d time.Duration) *Client {
	c.http.Timeout = d
	return c
}

func (c *Client) WithHeader(k, v string) *Client {
	c.headers[k] = v
	return c
}

func (c *Client) WithToken(token string) *Client {
	return c.WithHeader("Authorization", "Bearer "+token)
}

func (c *Client) Retry(times int, backoff time.Duration) *Client {
	c.retries = times
	if backoff > 0 {
		c.backoff = backoff
	}
	return c
}

// Do executes an HTTP request with configured headers and retry policy.
func (c *Client) Do(ctx context.Context, method, url string, body any) (*http.Response, error) {
	var rawBody []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rawBody = b
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		var reader io.Reader
		if rawBody != nil {
			reader = bytes.NewReader(rawBody)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return nil, err
		}
		if rawBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}

		resp, err := c.http.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		if resp != nil {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("httpclient: %s %s returned status %d", method, url, resp.StatusCode)
		} else {
			lastErr = err
		}
		if attempt < c.retries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.backoff * time.Duration(attempt+1)):
			}
		}
	}
	return nil, lastErr
}

// MaxResponseBytes caps JSON response decoding to prevent upstream memory exhaustion (default 10 MiB).
var MaxResponseBytes int64 = 10 << 20

// GetJSON performs a GET request on c and decodes the bounded JSON response into T (Go 1.27 generic method).
func (c *Client) GetJSON[T any](ctx context.Context, url string) (T, error) {
	return doJSON[T](ctx, c, http.MethodGet, url, nil)
}

// PostJSON performs a POST request on c with payload and decodes the bounded JSON response into T.
func (c *Client) PostJSON[T any](ctx context.Context, url string, payload any) (T, error) {
	return doJSON[T](ctx, c, http.MethodPost, url, payload)
}

// GetJSON delegates to c.GetJSON[T] (kept for backwards compatibility).
func GetJSON[T any](ctx context.Context, c *Client, url string) (T, error) {
	return c.GetJSON[T](ctx, url)
}

// PostJSON delegates to c.PostJSON[T] (kept for backwards compatibility).
func PostJSON[T any](ctx context.Context, c *Client, url string, payload any) (T, error) {
	return c.PostJSON[T](ctx, url, payload)
}

func doJSON[T any](ctx context.Context, c *Client, method, url string, payload any) (T, error) {
	var out T
	resp, err := c.Do(ctx, method, url, payload)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("httpclient: %s %s failed with status %d", method, url, resp.StatusCode)
	}
	limit := MaxResponseBytes
	if limit <= 0 {
		limit = 10 << 20
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, limit)).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}
