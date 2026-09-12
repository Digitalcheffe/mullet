// Package email sends outgoing mail over SMTP (issue #78) -- currently
// just password-reset links and the Settings page's test-email button.
// Deliberately stdlib-only (net/smtp + crypto/tls): a self-hosted tool
// shouldn't need a third-party mail API or SDK dependency just to relay
// through whatever SMTP server the admin already has (their own
// provider, a household NAS, a local Postfix relay).
package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
)

// TLS modes, matching db.SMTPTLSNone/SMTPTLSStartTLS/SMTPTLSTLS --
// duplicated as plain strings here rather than importing internal/db
// (this package has no other reason to depend on it, and the value
// just passes through as a string from Config).
const (
	TLSNone     = "none"
	TLSStartTLS = "starttls"
	TLSTLS      = "tls"
)

// Config is everything needed to send one message.
type Config struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	// TLSMode is one of TLSNone/TLSStartTLS/TLSTLS. Anything else is
	// treated as TLSStartTLS, the most common default (port 587).
	TLSMode string
}

// Send connects to cfg's SMTP server and sends a plain-text message to
// to. TLSMode controls the connection: TLSTLS dials with implicit TLS
// from the first byte (port 465 convention), TLSStartTLS dials
// plaintext and upgrades via STARTTLS if the server offers it (port 587
// convention), and TLSNone never encrypts the connection at all -- only
// appropriate for a trusted local relay reachable over a private
// network.
func Send(cfg Config, to, subject, body string) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	var conn net.Conn
	var err error
	if cfg.TLSMode == TLSTLS {
		conn, err = tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host})
	} else {
		conn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("starting smtp session: %w", err)
	}
	defer client.Close()

	if cfg.TLSMode != TLSNone && cfg.TLSMode != TLSTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}

	if cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
				return fmt.Errorf("authenticating: %w", err)
			}
		}
	}

	if err := client.Mail(cfg.FromAddress); err != nil {
		return fmt.Errorf("setting sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("setting recipient: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("opening message body: %w", err)
	}
	message := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		cfg.FromAddress, to, subject, body,
	)
	if _, err := wc.Write([]byte(message)); err != nil {
		wc.Close()
		return fmt.Errorf("writing message: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("finishing message: %w", err)
	}

	return client.Quit()
}
