package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strings"
)

// SMTPDriver sends MIME-compliant emails (text, HTML, multipart/alternative, and attachments)
// using Go's standard library net/smtp.
type SMTPDriver struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func (s SMTPDriver) Send(_ context.Context, msg Message) error {
	from := msg.From
	if from == "" {
		from = s.From
	}
	raw, err := buildMIME(from, msg)
	if err != nil {
		return err
	}

	recipients := make([]string, 0, len(msg.To)+len(msg.CC)+len(msg.BCC))
	recipients = append(recipients, msg.To...)
	recipients = append(recipients, msg.CC...)
	recipients = append(recipients, msg.BCC...)

	if strings.ContainsAny(from, "\r\n") {
		return fmt.Errorf("mail: invalid sender address")
	}
	for _, rcpt := range recipients {
		if strings.ContainsAny(rcpt, "\r\n") {
			return fmt.Errorf("mail: invalid recipient address")
		}
	}

	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	return smtp.SendMail(addr, auth, from, recipients, raw)
}

func buildMIME(from string, msg Message) ([]byte, error) {
	var buf bytes.Buffer
	crlf := strings.NewReplacer("\r", "", "\n", "")

	writeHeader := func(k, v string) {
		k = crlf.Replace(k)
		v = crlf.Replace(v)
		fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
	}

	writeHeader("From", from)
	if len(msg.To) > 0 {
		writeHeader("To", strings.Join(msg.To, ", "))
	}
	if len(msg.CC) > 0 {
		writeHeader("Cc", strings.Join(msg.CC, ", "))
	}
	if msg.ReplyTo != "" {
		writeHeader("Reply-To", msg.ReplyTo)
	}
	writeHeader("Subject", msg.Subject)
	writeHeader("MIME-Version", "1.0")
	for k, v := range msg.Headers {
		writeHeader(k, v)
	}

	// Simple single-part body when no attachments and only one body format.
	if len(msg.Attachments) == 0 && (msg.Text == "" || msg.HTML == "") {
		if msg.HTML != "" {
			writeHeader("Content-Type", "text/html; charset=UTF-8")
			buf.WriteString("\r\n" + msg.HTML)
		} else {
			writeHeader("Content-Type", "text/plain; charset=UTF-8")
			buf.WriteString("\r\n" + msg.Text)
		}
		return buf.Bytes(), nil
	}

	// Multipart MIME (mixed for attachments, alternative for text + HTML).
	outer := multipart.NewWriter(&buf)
	outerType := "multipart/alternative"
	if len(msg.Attachments) > 0 {
		outerType = "multipart/mixed"
	}
	writeHeader("Content-Type", fmt.Sprintf("%s; boundary=%s", outerType, outer.Boundary()))
	buf.WriteString("\r\n")

	if msg.Text != "" {
		h := textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}}
		p, err := outer.CreatePart(h)
		if err != nil {
			return nil, err
		}
		_, _ = p.Write([]byte(msg.Text))
	}
	if msg.HTML != "" {
		h := textproto.MIMEHeader{"Content-Type": {"text/html; charset=UTF-8"}}
		p, err := outer.CreatePart(h)
		if err != nil {
			return nil, err
		}
		_, _ = p.Write([]byte(msg.HTML))
	}

	for _, att := range msg.Attachments {
		safeName := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(att.Filename)
		h := textproto.MIMEHeader{
			"Content-Type":              {att.ContentType},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Disposition":       {fmt.Sprintf(`attachment; filename="%s"`, safeName)},
		}
		p, err := outer.CreatePart(h)
		if err != nil {
			return nil, err
		}
		enc := base64.StdEncoding.EncodeToString(att.Data)
		_, _ = p.Write([]byte(enc))
	}

	if err := outer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
