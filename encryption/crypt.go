package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypter provides AES-256-GCM authenticated encryption.
type Encrypter struct {
	aead cipher.AEAD
}

// GenerateKey returns a cryptographically random 32-byte key encoded as base64.
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(key), nil
}

// NewEncrypter initializes an AES-256-GCM Encrypter from a 32-byte raw or base64url key.
func NewEncrypter(key string) (*Encrypter, error) {
	raw := []byte(key)
	if len(raw) != 32 {
		decoded, err := base64.RawURLEncoding.DecodeString(key)
		if err != nil || len(decoded) != 32 {
			return nil, errors.New("crypt: key must be 32 raw bytes or base64url-encoded 32 bytes")
		}
		raw = decoded
	}

	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encrypter{aead: aead}, nil
}

// Encrypt encrypts plaintext into a base64url-encoded nonce+ciphertext string.
func (e *Encrypter) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := e.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt authenticates and decrypts a base64url-encoded payload produced by Encrypt.
func (e *Encrypter) Decrypt(payload string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("crypt: invalid payload encoding")
	}
	ns := e.aead.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("crypt: payload too short")
	}
	nonce, ciphertext := raw[:ns], raw[ns:]
	plain, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("crypt: decryption failed")
	}
	return plain, nil
}
