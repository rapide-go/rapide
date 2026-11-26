package storage

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rapide-go/rapide/support/option"
)

// S3Config configures any S3-compatible object store:
// AWS S3, MinIO (set Endpoint + UsePathStyle: true), Cloudflare R2, DigitalOcean Spaces, Backblaze B2.
type S3Config struct {
	Endpoint     string `env:"AWS_ENDPOINT"`
	Region       string `env:"AWS_DEFAULT_REGION" default:"us-east-1"`
	Bucket       string `env:"AWS_BUCKET"`
	AccessKey    string `env:"AWS_ACCESS_KEY_ID"`
	SecretKey    string `env:"AWS_SECRET_ACCESS_KEY"`
	UsePathStyle bool   `env:"AWS_USE_PATH_STYLE_ENDPOINT" default:"false"`
	Timeout      time.Duration
}

// S3 is a zero-dependency Disk driver using AWS Signature V4 over net/http.
type S3 struct {
	cfg    S3Config
	client *http.Client
}

// NewS3 constructs an S3-compatible Disk driver, applying optional generic option.Option[S3Config].
func NewS3(cfg S3Config, opts ...option.Option[S3Config]) *S3 {
	option.Apply(&cfg, opts...)
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &S3{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

func (s *S3) Put(path string, data []byte) error {
	resp, err := s.do(http.MethodPut, path, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("storage.s3: PUT %s failed with status %d", path, resp.StatusCode)
	}
	return nil
}

func (s *S3) Get(path string) ([]byte, error) {
	resp, err := s.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("storage.s3: GET %s failed with status %d", path, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *S3) Exists(path string) bool {
	resp, err := s.do(http.MethodHead, path, nil)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (s *S3) Delete(path string) error {
	resp, err := s.do(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("storage.s3: DELETE %s failed with status %d", path, resp.StatusCode)
	}
	return nil
}

func (s *S3) objectURL(key string) (string, error) {
	key = strings.TrimLeft(key, "/")
	if s.cfg.Endpoint != "" {
		base := strings.TrimRight(s.cfg.Endpoint, "/")
		if s.cfg.UsePathStyle {
			return fmt.Sprintf("%s/%s/%s", base, s.cfg.Bucket, key), nil
		}
		u, err := url.Parse(base)
		if err != nil {
			return "", err
		}
		u.Host = s.cfg.Bucket + "." + u.Host
		u.Path = "/" + key
		return u.String(), nil
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.cfg.Bucket, s.cfg.Region, key), nil
}

func (s *S3) do(method, key string, body []byte) (*http.Response, error) {
	rawURL, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = int64(len(body))
	}
	s.signV4(req, body)
	return s.client.Do(req)
}

// signV4 signs an HTTP request with AWS Signature Version 4 (works for S3, MinIO, R2, Spaces, B2).
func (s *S3) signV4(req *http.Request, payload []byte) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	payloadHash := sha256Hex(payload)
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.URL.Host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalReq := strings.Join([]string{
		req.Method,
		canonicalURI,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.cfg.Region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalReq)),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+s.cfg.SecretKey), dateStamp)
	kRegion := hmacSHA256(kDate, s.cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKey, scope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
