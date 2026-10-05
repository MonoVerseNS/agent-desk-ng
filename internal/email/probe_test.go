package email

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTPServer is a deliberately tiny SMTP responder: just enough of the
// protocol for the probe to negotiate a connection. It exists so the probe can be
// exercised against a real socket and a real EHLO/STARTTLS exchange instead of a
// mock that would pass even if the transport code were wrong.
type fakeSMTPServer struct {
	listener    net.Listener
	advertised  []string
	greeting    string
	mu          sync.Mutex
	connections int
}

func startFakeSMTPServer(t *testing.T, advertised []string) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error = %v", err)
	}
	server := &fakeSMTPServer{listener: listener, advertised: advertised, greeting: "220 fake ESMTP ready"}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			server.mu.Lock()
			server.connections++
			server.mu.Unlock()
			go server.handle(conn)
		}
	}()
	return server
}

func (s *fakeSMTPServer) address() (string, int) {
	host, port, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		panic(err)
	}
	return host, atoiOrFail(port)
}

func atoiOrFail(value string) int {
	total := 0
	for _, r := range value {
		total = total*10 + int(r-'0')
	}
	return total
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	writer := bufio.NewWriter(conn)
	_, _ = writer.WriteString(s.greeting + "\r\n")
	_ = writer.Flush()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(strings.ToUpper(line), "EHLO"), strings.HasPrefix(strings.ToUpper(line), "HELO"):
			_, _ = writer.WriteString("250-" + s.listener.Addr().String() + "\r\n")
			for i, ext := range s.advertised {
				sep := "-"
				if i == len(s.advertised)-1 {
					sep = " "
				}
				_, _ = writer.WriteString("250" + sep + ext + "\r\n")
			}
			if len(s.advertised) == 0 {
				_, _ = writer.WriteString("250 SIZE 35882577\r\n")
			}
			_ = writer.Flush()
		case strings.HasPrefix(strings.ToUpper(line), "QUIT"):
			_, _ = writer.WriteString("221 Bye\r\n")
			_ = writer.Flush()
			return
		case strings.HasPrefix(strings.ToUpper(line), "STARTTLS"):
			_, _ = writer.WriteString("454 TLS not available\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(strings.ToUpper(line), "AUTH"):
			_, _ = writer.WriteString("535 Authentication credentials invalid\r\n")
			_ = writer.Flush()
		default:
			_, _ = writer.WriteString("250 OK\r\n")
			_ = writer.Flush()
		}
	}
}

func probeConfig(host string, port int) ClientConfig {
	return ClientConfig{
		Provider:   ProviderSMTP,
		SMTPHost:   host,
		SMTPPort:   port,
		SMTPUseTLS: false,
	}
}

func TestProbeSMTPConnectsAndReportsCapabilities(t *testing.T) {
	server := startFakeSMTPServer(t, []string{"PIPELINING", "8BITMIME", "SIZE 35882577"})
	host, port := server.address()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ProbeSMTP(ctx, probeConfig(host, port))
	if err != nil {
		t.Fatalf("ProbeSMTP() error = %v", err)
	}
	if result.Host != host || result.Port != port {
		t.Fatalf("probe reported %s:%d, want %s:%d", result.Host, result.Port, host, port)
	}
	if !contains(result.Capabilities, "8BITMIME") || !contains(result.Capabilities, "PIPELINING") {
		t.Fatalf("capabilities = %v, want the advertised extensions", result.Capabilities)
	}
	if contains(result.Capabilities, "STARTTLS") {
		t.Fatal("STARTTLS was not advertised but shows as a capability")
	}
	// A server with no credentials configured must not report an auth attempt.
	if result.AuthAttempted {
		t.Fatal("a binding with no credentials should not attempt auth")
	}
}

// The test-server escape hatch: a self-signed endpoint that advertises STARTTLS
// would otherwise fail verification and take the probe down with it. Here the
// fake refuses the upgrade outright, so the probe must name the transport step
// and still report what the server offered.
func TestProbeSMTPReportsSTARTTLSOfferedAndFailsTheUpgrade(t *testing.T) {
	server := startFakeSMTPServer(t, []string{"STARTTLS", "PIPELINING"})
	host, port := server.address()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ProbeSMTP(ctx, probeConfig(host, port))
	if err == nil {
		t.Fatal("expected the refused STARTTLS to be reported rather than treated as healthy")
	}
	if result == nil {
		t.Fatal("a failed probe must still return what it reached")
	}
	if !result.STARTTLSOffered {
		t.Fatal("STARTTLS was advertised but not reported")
	}
	if result.TLSEstablished {
		t.Fatal("TLS must not be reported as established when the upgrade was refused")
	}
	if !strings.Contains(err.Error(), "starttls") {
		t.Fatalf("error %q should name the transport step that failed", err)
	}
	if !strings.Contains(err.Error(), "allow-insecure") {
		t.Fatalf("error %q should point at the test-server setting", err)
	}
}

func TestProbeSMTPFailsWhenNothingIsListening(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error = %v", err)
	}
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split error = %v", err)
	}
	// Close immediately so the port is almost certainly free.
	_ = listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := ProbeSMTP(ctx, probeConfig(host, atoiOrFail(port))); err == nil {
		t.Fatal("expected an unreachable server to fail the probe")
	}
}

func TestProbeSMTPRequiresAHost(t *testing.T) {
	if _, err := ProbeSMTP(context.Background(), ClientConfig{Provider: ProviderSMTP}); err == nil {
		t.Fatal("expected a missing host to be reported instead of dialling nothing")
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
