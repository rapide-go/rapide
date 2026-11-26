package router

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	crypt "github.com/rapide-go/rapide/encryption"
)

// URL generates a URL path from a named route (Rapide route('posts.show',...) equivalent).
// Matching `{param}` placeholders are substituted into the path; remaining keys become sorted query parameters.
func (r *Router) URL(name string, params ...map[string]any) (string, error) {
	info, ok := r.Named(name)
	if !ok {
		return "", fmt.Errorf("router: named route %q not found", name)
	}

	var p map[string]any
	if len(params) > 0 && params[0] != nil {
		p = params[0]
	}

	used := make(map[string]bool, len(p))
	var out strings.Builder
	path := info.Path

	for i := 0; i < len(path); {
		if path[i] == '{' {
			end := strings.IndexByte(path[i:], '}')
			if end > 0 {
				rawKey := path[i+1 : i+end]
				key := strings.TrimSuffix(rawKey, "...")
				val, exists := p[key]
				if !exists {
					return "", fmt.Errorf("router: missing route parameter %q for route %q", key, name)
				}
				used[key] = true
				out.WriteString(url.PathEscape(fmt.Sprint(val)))
				i += end + 1
				continue
			}
		}
		out.WriteByte(path[i])
		i++
	}

	if len(p) > len(used) {
		extraKeys := make([]string, 0, len(p)-len(used))
		for k := range p {
			if !used[k] {
				extraKeys = append(extraKeys, k)
			}
		}
		slices.Sort(extraKeys)
		q := url.Values{}
		for _, k := range extraKeys {
			q.Set(k, fmt.Sprint(p[k]))
		}
		out.WriteByte('?')
		out.WriteString(q.Encode())
	}

	return out.String(), nil
}

// MustURL is like URL but panics if the route name or parameters are invalid.
func (r *Router) MustURL(name string, params ...map[string]any) string {
	u, err := r.URL(name, params...)
	if err != nil {
		panic(err)
	}
	return u
}

// SignedURL generates an HMAC-SHA256 signed URL for a named route
// .
func (r *Router) SignedURL(baseURL, name, secret string, ttl time.Duration, params ...map[string]any) (string, error) {
	rel, err := r.URL(name, params...)
	if err != nil {
		return "", err
	}
	full := strings.TrimRight(baseURL, "/") + rel
	return crypt.SignURL(full, secret, ttl)
}
