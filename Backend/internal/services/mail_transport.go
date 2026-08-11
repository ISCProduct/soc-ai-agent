package services

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/resend/resend-go/v3"
)

// mailMessage は送信バックエンド共通のメッセージ表現（#756）。
type mailMessage struct {
	From    string
	To      []string
	Subject string
	HTML    string
	Text    string
}

// mailTransport はメール送信バックエンド。
type mailTransport interface {
	Name() string
	Send(msg mailMessage) error
}

// logMailTransport は開発用。実送信せずログのみ。
type logMailTransport struct{}

func (t *logMailTransport) Name() string { return "log" }

func (t *logMailTransport) Send(msg mailMessage) error {
	bodyLen := len(msg.HTML)
	if bodyLen == 0 {
		bodyLen = len(msg.Text)
	}
	log.Printf("[EmailService] provider=log simulating send to %v subject=%q body=%d bytes\n", msg.To, msg.Subject, bodyLen)
	return nil
}

// smtpMailTransport は従来の SMTP 送信。
type smtpMailTransport struct {
	host     string
	port     int
	user     string
	password string
}

func (t *smtpMailTransport) Name() string { return "smtp" }

func (t *smtpMailTransport) Send(msg mailMessage) error {
	if t.host == "" || msg.From == "" {
		return (&logMailTransport{}).Send(msg)
	}
	contentType := "text/html; charset=UTF-8"
	body := msg.HTML
	if body == "" {
		contentType = "text/plain; charset=UTF-8"
		body = msg.Text
	}
	toHeader := strings.Join(msg.To, ", ")
	raw := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n\r\n%s",
		msg.From, toHeader, msg.Subject, contentType, body,
	)
	addr := fmt.Sprintf("%s:%d", t.host, t.port)
	auth := smtp.PlainAuth("", t.user, t.password, t.host)
	if err := smtp.SendMail(addr, auth, msg.From, msg.To, []byte(raw)); err != nil {
		return fmt.Errorf("smtp send failed: %w", err)
	}
	log.Printf("[EmailService] provider=smtp sent to %v subject=%q\n", msg.To, msg.Subject)
	return nil
}

// resendMailTransport は Resend API 送信。
type resendMailTransport struct {
	client *resend.Client
}

func (t *resendMailTransport) Name() string { return "resend" }

func (t *resendMailTransport) Send(msg mailMessage) error {
	if t.client == nil {
		return fmt.Errorf("resend client is not configured")
	}
	params := &resend.SendEmailRequest{
		From:    msg.From,
		To:      msg.To,
		Subject: msg.Subject,
	}
	if msg.HTML != "" {
		params.Html = msg.HTML
	}
	if msg.Text != "" {
		params.Text = msg.Text
	}
	if params.Html == "" && params.Text == "" {
		params.Text = "(empty)"
	}
	sent, err := t.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend send failed: %w", err)
	}
	id := ""
	if sent != nil {
		id = sent.Id
	}
	log.Printf("[EmailService] provider=resend sent to %v subject=%q id=%s\n", msg.To, msg.Subject, id)
	return nil
}

// recordingMailTransport はテスト用。送信内容を記録する。
type recordingMailTransport struct {
	messages []mailMessage
}

func (t *recordingMailTransport) Name() string { return "recording" }

func (t *recordingMailTransport) Send(msg mailMessage) error {
	t.messages = append(t.messages, msg)
	return nil
}
