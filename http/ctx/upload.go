package ctx

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/rapide-go/rapide/filesystem"
)

// DefaultMaxUploadBytes caps multipart file uploads (default 10 MiB).
var DefaultMaxUploadBytes int64 = 10 << 20

// UploadedFile represents a validated, bounded file uploaded via multipart/form-data.
type UploadedFile struct {
	Name        string
	Extension   string
	Size        int64
	ContentType string
	Data        []byte
}

// File parses a multipart upload field, caps its size, sniffs its real MIME type,
// and optionally validates allowed MIME types (e.g. "image/png", "image/jpeg").
func (c *Ctx) File(field string, allowedMIMEs ...string) (*UploadedFile, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, DefaultMaxUploadBytes)
	if err := c.Request.ParseMultipartForm(DefaultMaxUploadBytes); err != nil {
		return nil, Abort(http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
	}

	f, header, err := c.Request.FormFile(field)
	if err != nil {
		return nil, Abort(http.StatusBadRequest, fmt.Sprintf("missing file field %q", field))
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, DefaultMaxUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > DefaultMaxUploadBytes {
		return nil, Abort(http.StatusRequestEntityTooLarge, "uploaded file exceeds size limit")
	}

	mimeType := http.DetectContentType(data)
	if len(allowedMIMEs) > 0 && !slices.Contains(allowedMIMEs, mimeType) {
		return nil, Abort(http.StatusUnprocessableEntity, fmt.Sprintf("unsupported file type %q", mimeType))
	}

	return &UploadedFile{
		Name:        filepath.Base(header.Filename),
		Extension:   filepath.Ext(header.Filename),
		Size:        int64(len(data)),
		ContentType: mimeType,
		Data:        data,
	}, nil
}

// SaveFile parses an uploaded file field and stores it directly onto Rapide's storage.Storage() disk.
func (c *Ctx) SaveFile(field, destPath string, allowedMIMEs ...string) (*UploadedFile, error) {
	uf, err := c.File(field, allowedMIMEs...)
	if err != nil {
		return nil, err
	}
	if err := storage.Put(destPath, uf.Data); err != nil {
		return nil, err
	}
	return uf, nil
}
