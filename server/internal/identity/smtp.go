package identity

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Address, From, Username, Password string
	AllowLocalInsecure                bool
}

func SMTPMailer(config SMTPConfig) func(context.Context, string, string) error {
	return func(ctx context.Context, to, link string) error {
		host, _, err := net.SplitHostPort(config.Address)
		if err != nil {
			return errors.New("invalid SMTP address")
		}
		from, err := mail.ParseAddress(config.From)
		if err != nil {
			return errors.New("invalid sender")
		}
		recipient, err := mail.ParseAddress(to)
		if err != nil || strings.ContainsAny(link, "\r\n") {
			return errors.New("invalid message")
		}
		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", config.Address)
		if err != nil {
			return errors.New("mail delivery unavailable")
		}
		defer func() { _ = conn.Close() }()
		deadline := time.Now().Add(8 * time.Second)
		if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		_ = conn.SetDeadline(deadline)
		stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
		defer stop()
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return errors.New("mail delivery unavailable")
		}
		defer func() { _ = client.Close() }()
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return errors.New("SMTP TLS failed")
			}
		} else {
			ip := net.ParseIP(host)
			if !config.AllowLocalInsecure || ip == nil || !ip.IsLoopback() {
				return errors.New("SMTP requires STARTTLS")
			}
		}
		if config.Username != "" {
			if err = client.Auth(smtp.PlainAuth("", config.Username, config.Password, host)); err != nil {
				return errors.New("SMTP authentication failed")
			}
		}
		if err = client.Mail(from.Address); err != nil {
			return errors.New("mail delivery unavailable")
		}
		if err = client.Rcpt(recipient.Address); err != nil {
			return errors.New("mail delivery unavailable")
		}
		writer, err := client.Data()
		if err != nil {
			return errors.New("mail delivery unavailable")
		}
		_, err = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n请打开下面的链接验证 PluginPocket 邮箱，链接 30 分钟内有效：\r\n%s\r\n", from.String(), recipient.String(), mime.QEncoding.Encode("utf-8", "验证你的 PluginPocket 邮箱"), link)
		if err != nil {
			return errors.New("mail delivery unavailable")
		}
		if err = writer.Close(); err != nil {
			return errors.New("mail delivery unavailable")
		}
		return client.Quit()
	}
}
