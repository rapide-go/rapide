package mail

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// Attachment represents a file attached to an outbound email.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Message represents an outbound email built directly or fluently.
type Message struct {
	From        string
	To          []string
	CC          []string
	BCC         []string
	ReplyTo     string
	Subject     string
	Text        string
	HTML        string
	Headers     map[string]string
	Attachments []Attachment
}

// NewMessage starts a fluent email builder.
func NewMessage() *Message {
	return &Message{Headers: make(map[string]string)}
}

func (m *Message) SetFrom(addr string) *Message    { m.From = addr; return m }
func (m *Message) AddTo(addrs ...string) *Message  { m.To = append(m.To, addrs...); return m }
func (m *Message) AddCC(addrs ...string) *Message  { m.CC = append(m.CC, addrs...); return m }
func (m *Message) AddBCC(addrs ...string) *Message { m.BCC = append(m.BCC, addrs...); return m }
func (m *Message) SetReplyTo(addr string) *Message { m.ReplyTo = addr; return m }
func (m *Message) SetSubject(s string) *Message    { m.Subject = s; return m }
func (m *Message) SetText(body string) *Message    { m.Text = body; return m }
func (m *Message) SetHTML(body string) *Message    { m.HTML = body; return m }

func (m *Message) SetHeader(k, v string) *Message {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[k] = v
	return m
}

// AttachData attaches raw bytes with an auto-detected or explicit content type.
func (m *Message) AttachData(filename string, data []byte, contentType ...string) *Message {
	ct := ""
	if len(contentType) > 0 && contentType[0] != "" {
		ct = contentType[0]
	} else {
		ct = http.DetectContentType(data)
	}
	m.Attachments = append(m.Attachments, Attachment{
		Filename:    filepath.Base(filename),
		ContentType: ct,
		Data:        data,
	})
	return m
}

// AttachFile reads a file from disk and attaches it to the message.
func (m *Message) AttachFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m.AttachData(filepath.Base(path), data)
	return nil
}

// RenderHTML executes an embedded html/template file and sets the message's HTML body.
func (m *Message) RenderHTML(fsys fs.FS, pattern, name string, data any) error {
	tpl, err := template.ParseFS(fsys, pattern)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	m.HTML = buf.String()
	return nil
}

// Mailable is Rapide's typed Mailable contract: any struct that builds a Message.
type Mailable interface {
	Build() (Message, error)
}
