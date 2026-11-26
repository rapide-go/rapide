package socialite

import "fmt"

// GitHub returns a pre-configured GitHub OAuth2 Provider.
func GitHub(clientID, clientSecret, redirectURI string) *Provider {
	return &Provider{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"read:user", "user:email"},
		ParseUser: func(raw map[string]any) User {
			return User{
				ID:     fmt.Sprint(raw["id"]),
				Name:   strField(raw, "name", "login"),
				Email:  strField(raw, "email"),
				Avatar: strField(raw, "avatar_url"),
				Raw:    raw,
			}
		},
	}
}

// Google returns a pre-configured Google OAuth2 Provider.
func Google(clientID, clientSecret, redirectURI string) *Provider {
	return &Provider{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		Scopes:       []string{"openid", "email", "profile"},
		ParseUser: func(raw map[string]any) User {
			return User{
				ID:     strField(raw, "id", "sub"),
				Name:   strField(raw, "name"),
				Email:  strField(raw, "email"),
				Avatar: strField(raw, "picture"),
				Raw:    raw,
			}
		},
	}
}

// Discord returns a pre-configured Discord OAuth2 Provider.
func Discord(clientID, clientSecret, redirectURI string) *Provider {
	return &Provider{
		Name:         "discord",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		AuthURL:      "https://discord.com/api/oauth2/authorize",
		TokenURL:     "https://discord.com/api/oauth2/token",
		UserInfoURL:  "https://discord.com/api/users/@me",
		Scopes:       []string{"identify", "email"},
		ParseUser: func(raw map[string]any) User {
			return User{
				ID:     strField(raw, "id"),
				Name:   strField(raw, "global_name", "username"),
				Email:  strField(raw, "email"),
				Avatar: strField(raw, "avatar"),
				Raw:    raw,
			}
		},
	}
}

func strField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
