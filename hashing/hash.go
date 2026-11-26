package hash

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Hasher is the contract for password hashing drivers (PBKDF2 stdlib default; swap for Argon2/Bcrypt).
type Hasher interface {
	Make(password string) (string, error)
	Check(password, encoded string) bool
}

// PBKDF2 implements Hasher using Go's standard library crypto/pbkdf2 with SHA-256.
type PBKDF2 struct {
	Iterations int
	KeyLen     int
	SaltLen    int
}

// Default is the built-in PBKDF2-SHA256 hasher (210,000 iterations per OWASP guidance).
var Default Hasher = PBKDF2{
	Iterations: 210_000,
	KeyLen:     32,
	SaltLen:    16,
}

// Make hashes password using the Default hasher.
func Make(password string) (string, error) {
	return Default.Make(password)
}

// Check verifies password against an encoded hash using the Default hasher.
func Check(password, encoded string) bool {
	return Default.Check(password, encoded)
}

func (p PBKDF2) Make(password string) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, p.Iterations, p.KeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"pbkdf2-sha256$%d$%s$%s",
		p.Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

const (
	minPBKDF2Iterations = 10_000
	maxPBKDF2Iterations = 5_000_000
)

// NeedsRehash reports whether an encoded hash should be re-hashed with current Default settings.
func NeedsRehash(encoded string) bool {
	p, ok := Default.(PBKDF2)
	if !ok {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return true
	}
	iters, err := strconv.Atoi(parts[1])
	return err != nil || iters < p.Iterations
}

func (p PBKDF2) Check(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iters, err := strconv.Atoi(parts[1])
	if err != nil || iters < minPBKDF2Iterations || iters > maxPBKDF2Iterations {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 || len(expected) > 64 {
		return false
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, iters, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(dk, expected) == 1
}
