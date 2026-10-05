package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SMTPProbeResult describes what a bound SMTP server actually offers, so an
// operator can tell "wrong host" from "reachable but will not accept our
// credentials" from "needs TLS and we are not using it".
type SMTPProbeResult struct {
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	Provider        string   `json:"provider"`
	Greeting        string   `json:"greeting"`
	Capabilities    []string `json:"capabilities"`
	STARTTLSOffered bool     `json:"starttlsOffered"`
	TLSEstablished  bool     `json:"tlsEstablished"`
	AuthAccepted    bool     `json:"authAccepted"`
	// AuthAttempted is false when the binding carries no credentials, which is
	// the normal shape of a local test server.
	AuthAttempted bool `json:"authAttempted"`
}

// ProbeSMTP opens a connection to the configured server, negotiates the same
// transport the sender would use, and reports what it found. It sends no mail.
//
// The probe deliberately reuses the sender's transport rules rather than a
// friendlier variant: a probe that succeeds by being more permissive than the
// real send would let a binding verify as healthy while every message fails.
func ProbeSMTP(ctx context.Context, cfg ClientConfig) (*SMTPProbeResult, error) {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return nil, fmt.Errorf("smtp host is not configured")
	}
	port := cfg.SMTPPort
	if port <= 0 {
		port = 587
	}
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(port))

	timeout := 15 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	dialer := &net.Dialer{Timeout: timeout}

	result := &SMTPProbeResult{
		Host:     cfg.SMTPHost,
		Port:     port,
		Provider: string(cfg.Provider),
	}

	var (
		conn          net.Conn
		client        *smtp.Client
		tlsConfigured bool
		err           error
	)

	if port == 465 || cfg.SMTPUseTLS {
		tlsConfigured = true
		tlsConfig := &tls.Config{ServerName: cfg.SMTPHost}
		if cfg.SMTPAllowInsecure {
			tlsConfig.InsecureSkipVerify = true
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		if err != nil {
			return result, fmt.Errorf("failed to connect via tls: %w", err)
		}
		result.TLSEstablished = true
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return result, fmt.Errorf("failed to dial smtp: %w", err)
		}
	}

	client, err = smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return result, fmt.Errorf("failed to create smtp client: %w", err)
	}
	defer func() {
		_ = client.Quit()
	}()

	// net/smtp exposes no extension lister, so the capabilities that actually
	// affect delivery are probed by name. A raw list would be noise to an operator.
	if err = client.Hello("localhost"); err != nil {
		_ = conn.Close()
		return result, fmt.Errorf("smtp greeting failed: %w", err)
	}

	if ext, _ := client.Extension("STARTTLS"); ext {
		result.STARTTLSOffered = true
		if !tlsConfigured && !cfg.SMTPAllowInsecure {
			tlsConfig := &tls.Config{ServerName: cfg.SMTPHost}
			if err = client.StartTLS(tlsConfig); err != nil {
				return result, fmt.Errorf("starttls failed (enable allow-insecure for a self-signed test server): %w", err)
			}
			result.TLSEstablished = true
		}
	}

	capabilities := make([]string, 0, 6)
	for _, ext := range []string{"8BITMIME", "SMTPUTF8", "PIPELINING", "SIZE", "ENHANCEDSTATUSCODES", "AUTH PLAIN"} {
		if ok, _ := client.Extension(ext); ok {
			capabilities = append(capabilities, strings.ToUpper(ext))
		}
	}
	sort.Strings(capabilities)
	result.Capabilities = capabilities

	if strings.TrimSpace(cfg.SMTPUser) != "" && strings.TrimSpace(cfg.SMTPPassword) != "" {
		result.AuthAttempted = true
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
			// PlainAuth itself refuses to send credentials in the clear, which would
			// report as an auth failure rather than as the transport problem it is.
			if !result.TLSEstablished && !isLocalhostHost(cfg.SMTPHost) {
				return result, fmt.Errorf(
					"refusing to authenticate over an unencrypted connection: enable TLS, or allow-insecure for a local test server")
			}
			if err = client.Auth(auth); err != nil {
				return result, fmt.Errorf("smtp auth failed: %w", err)
			}
			result.AuthAccepted = true
		}
	}

	return result, nil
}

func isLocalhostHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if i := strings.LastIndex(h, ":"); i >= 0 && !strings.Contains(h, "]") {
		h = h[:i]
	}
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}
