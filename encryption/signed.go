package crypt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"time"
)

// SignURL appends an HMAC-SHA256 `expires` and `signature` query parameter to rawURL
// .
func SignURL(rawURL, secret string, ttl time.Duration) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if ttl > 0 {
		q.Set("expires", strconv.FormatInt(time.Now().Add(ttl).Unix(), 10))
	}
	u.RawQuery = q.Encode()

	sig := computeHMAC(u.String(), secret)
	q.Set("signature", sig)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// VerifyURL validates the HMAC signature and optional expiration timestamp of a signed URL.
func VerifyURL(rawURL, secret string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	q := u.Query()
	sig := q.Get("signature")
	if sig == "" {
		return errors.New("crypt: missing url signature")
	}

	if expStr := q.Get("expires"); expStr != "" {
		exp, err := strconv.ParseInt(expStr, 10, 64)
		if err != nil || time.Now().Unix() > exp {
			return errors.New("crypt: signed url has expired")
		}
	}

	q.Del("signature")
	u.RawQuery = q.Encode()
	expected := computeHMAC(u.String(), secret)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return errors.New("crypt: invalid url signature")
	}
	return nil
}

func computeHMAC(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
