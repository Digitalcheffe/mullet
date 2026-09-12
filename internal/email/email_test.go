package email

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTPServer is a minimal plaintext SMTP server good enough to
// exercise Send's happy path end-to-end (real dial, real command
// sequence) without depending on an external mail server or mocking
// net/smtp itself. It only understands the handful of commands Send
// actually issues -- EHLO, AUTH PLAIN (optional), MAIL FROM, RCPT TO,
// DATA, QUIT -- and records the message it received for the test to
// assert against.
type fakeSMTPServer struct {
	Addr string

	mu          sync.Mutex
	lastMessage string
	authSeen    bool
}

func startFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting fake smtp server: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	srv := &fakeSMTPServer{Addr: listener.Addr().String()}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // listener closed by t.Cleanup
			}
			go srv.handle(conn)
		}
	}()

	return srv
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	fmt.Fprint(conn, "220 fake.smtp ESMTP\r\n")

	var data strings.Builder
	inData := false
	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")

		if inData {
			if trimmed == "." {
				inData = false
				s.mu.Lock()
				s.lastMessage = data.String()
				s.mu.Unlock()
				fmt.Fprint(conn, "250 OK\r\n")
				continue
			}
			data.WriteString(trimmed + "\n")
			continue
		}

		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			fmt.Fprint(conn, "250-fake.smtp\r\n250 AUTH PLAIN\r\n")
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			s.mu.Lock()
			s.authSeen = true
			s.mu.Unlock()
			fmt.Fprint(conn, "235 OK\r\n")
		case strings.HasPrefix(upper, "MAIL FROM"):
			fmt.Fprint(conn, "250 OK\r\n")
		case strings.HasPrefix(upper, "RCPT TO"):
			fmt.Fprint(conn, "250 OK\r\n")
		case upper == "DATA":
			inData = true
			fmt.Fprint(conn, "354 Send message, end with <CRLF>.<CRLF>\r\n")
		case upper == "QUIT":
			fmt.Fprint(conn, "221 Bye\r\n")
			return
		default:
			fmt.Fprint(conn, "500 unrecognized\r\n")
		}
	}
}

func (s *fakeSMTPServer) LastMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMessage
}

func (s *fakeSMTPServer) AuthSeen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authSeen
}

func TestSendDeliversMessageOverPlainConnection(t *testing.T) {
	srv := startFakeSMTPServer(t)
	host, portStr, _ := net.SplitHostPort(srv.Addr)
	port, _ := strconv.Atoi(portStr)

	err := Send(Config{
		Host: host, Port: port, Username: "mullet", Password: "s3cret",
		FromAddress: "mullet@example.com", TLSMode: TLSNone,
	}, "admin@example.com", "Test subject", "Test body")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	msg := srv.LastMessage()
	if !strings.Contains(msg, "Subject: Test subject") {
		t.Errorf("message missing subject header, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Test body") {
		t.Errorf("message missing body, got:\n%s", msg)
	}
	if !strings.Contains(msg, "To: admin@example.com") {
		t.Errorf("message missing To header, got:\n%s", msg)
	}
	if !srv.AuthSeen() {
		t.Error("Send with a non-empty Username never authenticated")
	}
}

func TestSendWithoutCredentialsSkipsAuth(t *testing.T) {
	srv := startFakeSMTPServer(t)
	host, portStr, _ := net.SplitHostPort(srv.Addr)
	port, _ := strconv.Atoi(portStr)

	err := Send(Config{
		Host: host, Port: port, FromAddress: "mullet@example.com", TLSMode: TLSNone,
	}, "admin@example.com", "Subject", "Body")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if srv.AuthSeen() {
		t.Error("Send with no Username authenticated anyway")
	}
}

func TestSendConnectionRefusedReturnsError(t *testing.T) {
	// Nothing listens here -- Send must return an error, not hang or panic.
	err := Send(Config{Host: "127.0.0.1", Port: 1, FromAddress: "a@example.com", TLSMode: TLSNone}, "b@example.com", "s", "b")
	if err == nil {
		t.Error("Send to a closed port = nil error, want a connection error")
	}
}
