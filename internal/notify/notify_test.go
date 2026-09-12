package notify

import (
	"bufio"
	"database/sql"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// fakeSMTPServer is a minimal plaintext SMTP server, good enough to
// exercise broadcast's fan-out to multiple recipients -- a smaller
// local duplicate of internal/email's own fake server (Go test helpers
// aren't exported across packages), except this one records every
// message it receives rather than just the last one, since these tests
// care about how many recipients actually got mail.
type fakeSMTPServer struct {
	Addr string

	mu       sync.Mutex
	messages []string
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
				s.messages = append(s.messages, data.String())
				s.mu.Unlock()
				data.Reset()
				fmt.Fprint(conn, "250 OK\r\n")
				continue
			}
			data.WriteString(trimmed + "\n")
			continue
		}

		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			fmt.Fprint(conn, "250 fake.smtp\r\n")
		case strings.HasPrefix(upper, "MAIL FROM"), strings.HasPrefix(upper, "RCPT TO"):
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

func (s *fakeSMTPServer) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.messages...)
}

// newTestDB returns a migrated database with SMTP already pointed at a
// fresh fake server, ready for a test to seed admins and preferences
// into.
func newTestDB(t *testing.T) (*sql.DB, *fakeSMTPServer) {
	t.Helper()
	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	srv := startFakeSMTPServer(t)
	host, portStr, _ := net.SplitHostPort(srv.Addr)
	port, _ := strconv.Atoi(portStr)
	if err := db.SaveSMTPConfig(sqldb, db.SMTPConfig{
		Host: host, Port: port, FromAddress: "mullet@example.com", TLSMode: db.SMTPTLSNone,
	}); err != nil {
		t.Fatalf("SaveSMTPConfig: %v", err)
	}

	return sqldb, srv
}

// seedAdmin creates an admin account with the given email (empty means
// no email on file) and returns its username.
func seedAdmin(t *testing.T, sqldb *sql.DB, username, email string) {
	t.Helper()
	user, err := db.CreateUser(sqldb, username, "hash")
	if err != nil {
		t.Fatalf("CreateUser(%q): %v", username, err)
	}
	if email != "" {
		if err := db.SetUserEmail(sqldb, user.ID, email); err != nil {
			t.Fatalf("SetUserEmail(%q): %v", username, err)
		}
	}
}

func TestBroadcastNoopsWithoutSMTPConfigured(t *testing.T) {
	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	seedAdmin(t, sqldb, "admin", "admin@example.com")
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{NewUser: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	if err := NewUser(sqldb, "someone"); err != nil {
		t.Errorf("NewUser without SMTP configured = %v, want nil (graceful no-op)", err)
	}
}

func TestBroadcastNoopsWhenPreferenceDisabled(t *testing.T) {
	sqldb, srv := newTestDB(t)
	seedAdmin(t, sqldb, "admin", "admin@example.com")
	// Preferences default to all-off -- never explicitly enabled here.

	if err := NewUser(sqldb, "someone"); err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if len(srv.Messages()) != 0 {
		t.Errorf("Messages = %d, want 0 (preference is off)", len(srv.Messages()))
	}
}

func TestNewUserSendsToEveryAdminWithEmail(t *testing.T) {
	sqldb, srv := newTestDB(t)
	seedAdmin(t, sqldb, "alice", "alice@example.com")
	seedAdmin(t, sqldb, "bob", "bob@example.com")
	seedAdmin(t, sqldb, "carol", "") // no email on file -- must be skipped
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{NewUser: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	if err := NewUser(sqldb, "dave"); err != nil {
		t.Fatalf("NewUser: %v", err)
	}

	msgs := srv.Messages()
	if len(msgs) != 2 {
		t.Fatalf("Messages = %d, want 2 (alice and bob only)", len(msgs))
	}
	joined := strings.Join(msgs, "\n---\n")
	if !strings.Contains(joined, "To: alice@example.com") || !strings.Contains(joined, "To: bob@example.com") {
		t.Errorf("messages = %s, want one addressed to alice and one to bob", joined)
	}
	if !strings.Contains(joined, "dave") {
		t.Errorf("messages = %s, want the new username mentioned", joined)
	}
}

func TestEventFunctionsRespectTheirOwnPreference(t *testing.T) {
	tests := []struct {
		name        string
		setPrefs    db.NotificationPreferences
		call        func(sqldb *sql.DB) error
		wantSubject string
	}{
		{"NewUser", db.NotificationPreferences{NewUser: true},
			func(sqldb *sql.DB) error { return NewUser(sqldb, "alice") }, "Subject: New admin account created"},
		{"PasswordResetRequested", db.NotificationPreferences{PasswordResetRequested: true},
			func(sqldb *sql.DB) error { return PasswordResetRequested(sqldb, "alice") }, "Subject: Password reset requested"},
		{"PasswordResetCompleted", db.NotificationPreferences{PasswordResetCompleted: true},
			func(sqldb *sql.DB) error { return PasswordResetCompleted(sqldb, "alice") }, "Subject: Password reset completed"},
		{"ClientRegistered", db.NotificationPreferences{ClientRegistered: true},
			func(sqldb *sql.DB) error { return ClientRegistered(sqldb, "Living Room TV") }, "Subject: New client pending approval"},
		{"ClientApproved", db.NotificationPreferences{ClientApproved: true},
			func(sqldb *sql.DB) error { return ClientApproved(sqldb, "Living Room TV") }, "Subject: Client approved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqldb, srv := newTestDB(t)
			seedAdmin(t, sqldb, "admin", "admin@example.com")
			if err := db.SaveNotificationPreferences(sqldb, tt.setPrefs); err != nil {
				t.Fatalf("SaveNotificationPreferences: %v", err)
			}

			if err := tt.call(sqldb); err != nil {
				t.Fatalf("call: %v", err)
			}

			msgs := srv.Messages()
			if len(msgs) != 1 {
				t.Fatalf("Messages = %d, want exactly 1", len(msgs))
			}
			if !strings.Contains(msgs[0], tt.wantSubject) {
				t.Errorf("message = %s, want subject containing %q", msgs[0], tt.wantSubject)
			}
		})
	}
}
