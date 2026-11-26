package socialite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	httpclient "github.com/rapide-go/rapide/http/client"
	"github.com/rapide-go/rapide/support/manager"
)

// User is the normalized OAuth2 user profile returned by any Socialite provider.
type User struct {
	ID          string
	Name        string
	Email       string
	Avatar      string
	AccessToken string
	Raw         map[string]any
}

// Provider configures a generic OAuth2 social login driver.
type Provider struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	HTTPClient   *http.Client
	ParseUser    func(raw map[string]any) User
}

// Manager returns the multi-provider *manager.Manager[*Provider] bound in the container.
func Manager() *manager.Manager[*Provider] {
	return container.Use[*manager.Manager[*Provider]]()
}

// Driver resolves a named OAuth provider (e.g. socialite.Driver("github")).
func Driver(name string) *Provider {
	return Manager().Must(name)
}

// Extend registers a custom OAuth provider factory under name.
func Extend(name string, fn manager.Factory[*Provider]) {
	Manager().Extend(name, fn)
}

// ServiceProvider binds named Socialite OAuth providers into the Rapide container.
type ServiceProvider struct {
	foundation.BaseProvider
	Drivers map[string]*Provider
}

func (sp ServiceProvider) Register(app *foundation.App) error {
	m := manager.New[*Provider]()
	// Auto-register any OAuth drivers configured in config/services.go (or env).
	if id := config.Get("services.github_client_id", ""); id != "" {
		m.Set("github", GitHub(
			id,
			config.Get("services.github_client_secret", ""),
			config.Get("services.github_redirect_uri", ""),
		))
	}
	if id := config.Get("services.google_client_id", ""); id != "" {
		m.Set("google", Google(
			id,
			config.Get("services.google_client_secret", ""),
			config.Get("services.google_redirect_uri", ""),
		))
	}
	if id := config.Get("services.discord_client_id", ""); id != "" {
		m.Set("discord", Discord(
			id,
			config.Get("services.discord_client_secret", ""),
			config.Get("services.discord_redirect_uri", ""),
		))
	}
	for name, drv := range sp.Drivers {
		m.Set(name, drv)
	}
	container.Bind[*manager.Manager[*Provider]](app.Container, m)
	return nil
}

// GenerateState returns a cryptographically random OAuth2 CSRF state token.
func GenerateState() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AuthCodeURL constructs the provider authorization redirect URL for the given CSRF state.
func (p *Provider) AuthCodeURL(state string) string {
	q := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {p.RedirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(p.Scopes, " ")},
		"state":         {state},
	}
	return p.AuthURL + "?" + q.Encode()
}

// Exchange swaps an OAuth2 authorization code for an access token and fetches the User profile.
func (p *Provider) Exchange(ctx context.Context, code string) (User, error) {
	hc := p.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}

	form := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"code":          {code},
		"redirect_uri":  {p.RedirectURI},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("socialite: token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return User{}, fmt.Errorf("socialite: token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil || tokenResp.AccessToken == "" {
		return User{}, fmt.Errorf("socialite: missing access_token in response")
	}

	raw, err := httpclient.GetJSON[map[string]any](
		ctx,
		httpclient.New().WithToken(tokenResp.AccessToken).WithHeader("Accept", "application/json"),
		p.UserInfoURL,
	)
	if err != nil {
		return User{}, fmt.Errorf("socialite: userinfo request: %w", err)
	}

	var u User
	if p.ParseUser != nil {
		u = p.ParseUser(raw)
	} else {
		u = User{
			ID:     strField(raw, "id", "sub"),
			Name:   strField(raw, "name"),
			Email:  strField(raw, "email"),
			Avatar: strField(raw, "avatar", "picture"),
			Raw:    raw,
		}
	}
	u.AccessToken = tokenResp.AccessToken
	return u, nil
}
