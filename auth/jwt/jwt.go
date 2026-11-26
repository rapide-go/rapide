package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("jwt: invalid token")
	ErrExpiredToken = errors.New("jwt: token has expired")
	ErrNotYetValid  = errors.New("jwt: token is not valid yet")
)

var hs256Header = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// RegisteredClaims holds standard JWT timing and identity fields.
// Embed it in your own custom claims struct.
type RegisteredClaims struct {
	Subject   string `json:"sub,omitempty"`
	Issuer    string `json:"iss,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	NotBefore int64  `json:"nbf,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
}

// Expiring creates RegisteredClaims valid from now for ttl.
func Expiring(subject string, ttl time.Duration) RegisteredClaims {
	now := time.Now().Unix()
	return RegisteredClaims{
		Subject:   subject,
		IssuedAt:  now,
		ExpiresAt: now + int64(ttl.Seconds()),
	}
}

// Sign encodes claims of type C into a compact HS256 JWT signed with secret.
func Sign[C any](claims C, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("jwt: secret cannot be empty")
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := hs256Header + "." + payloadB64
	sig := signHS256(signingInput, secret)
	return signingInput + "." + sig, nil
}

// Verify checks the HS256 signature, validates exp/nbf timestamps if present,
// and decodes the payload into typed claims C.
func Verify[C any](token, secret string) (C, error) {
	var claims C
	if secret == "" {
		return claims, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, ErrInvalidToken
	}

	// Verify header specifies HS256 (prevents alg:none / algorithm confusion attacks).
	var hdr struct {
		Alg string `json:"alg"`
	}
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(hdrBytes, &hdr) != nil || hdr.Alg != "HS256" {
		return claims, ErrInvalidToken
	}

	expectedSig := signHS256(parts[0]+"."+parts[1], secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return claims, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, ErrInvalidToken
	}

	var timing RegisteredClaims
	if err := json.Unmarshal(payloadBytes, &timing); err != nil {
		return claims, ErrInvalidToken
	}
	now := time.Now().Unix()
	if timing.ExpiresAt > 0 && now > timing.ExpiresAt {
		return claims, ErrExpiredToken
	}
	if timing.NotBefore > 0 && now < timing.NotBefore {
		return claims, ErrNotYetValid
	}

	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return claims, ErrInvalidToken
	}
	return claims, nil
}

func signHS256(input, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
