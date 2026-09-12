// Package notify delivers a small, explicit set of security-relevant
// events (issue #112) to every configured admin -- not a general-
// purpose notification framework, just email and/or an outgoing
// webhook wired up to the handful of things worth knowing about on a
// self-hosted server nobody else is watching.
package notify

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/email"
)

// broadcastEmail emails every admin with an address on file, using
// whatever SMTP config is currently saved. It no-ops cleanly (returns
// nil, sending nothing) when SMTP isn't configured -- the same
// graceful-degradation expectation issue #78 already set for password
// reset -- so callers can fire a notification unconditionally without
// checking setup state themselves first.
func broadcastEmail(sqldb *sql.DB, subject, body string) error {
	cfg, found, err := db.GetSMTPConfig(sqldb)
	if err != nil {
		return fmt.Errorf("loading smtp config: %w", err)
	}
	if !found || cfg.Host == "" {
		return nil
	}

	users, err := db.ListUsers(sqldb)
	if err != nil {
		return fmt.Errorf("listing admins: %w", err)
	}

	var errs []error
	for _, u := range users {
		if u.Email == nil || *u.Email == "" {
			continue
		}
		if err := email.Send(email.Config{
			Host: cfg.Host, Port: cfg.Port, Username: cfg.Username, Password: cfg.Password,
			FromAddress: cfg.FromAddress, TLSMode: cfg.TLSMode,
		}, *u.Email, subject, body); err != nil {
			errs = append(errs, fmt.Errorf("notifying %s: %w", *u.Email, err))
		}
	}
	return errors.Join(errs...)
}

// dispatch delivers one event over every channel a preference toggle
// covers -- currently email and an outgoing webhook, both sharing the
// same on/off toggle rather than needing a separate one per channel.
// Both legs no-op cleanly when their own destination isn't configured,
// so enabling the toggle before setting up SMTP or a webhook URL is
// harmless, not an error.
func dispatch(sqldb *sql.DB, enabled bool, eventKey, subject, body string) error {
	if !enabled {
		return nil
	}
	var errs []error
	if err := broadcastEmail(sqldb, subject, body); err != nil {
		errs = append(errs, err)
	}
	if err := deliverWebhook(sqldb, eventKey, subject, body); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// NewUser notifies every admin that a new admin account was created.
func NewUser(sqldb *sql.DB, username string) error {
	prefs, err := db.GetNotificationPreferences(sqldb)
	if err != nil {
		return err
	}
	return dispatch(sqldb, prefs.NewUser, "new_user", "New admin account created",
		fmt.Sprintf("A new admin account was created on your Mullet server: %s\n\nIf you didn't expect this, review the Users section in Settings.", username))
}

// PasswordResetRequested notifies every admin that a password reset was
// requested for the named account -- separate from (and in addition to)
// the reset link itself, which only goes to that account's own email.
func PasswordResetRequested(sqldb *sql.DB, username string) error {
	prefs, err := db.GetNotificationPreferences(sqldb)
	if err != nil {
		return err
	}
	return dispatch(sqldb, prefs.PasswordResetRequested, "password_reset_requested", "Password reset requested",
		fmt.Sprintf("A password reset was requested for the admin account %q on your Mullet server.\n\nIf you didn't request this, no action is needed -- the reset link expires in an hour and only works once.", username))
}

// PasswordResetCompleted notifies every admin that a password reset was
// completed for the named account.
func PasswordResetCompleted(sqldb *sql.DB, username string) error {
	prefs, err := db.GetNotificationPreferences(sqldb)
	if err != nil {
		return err
	}
	return dispatch(sqldb, prefs.PasswordResetCompleted, "password_reset_completed", "Password reset completed",
		fmt.Sprintf("The password for admin account %q on your Mullet server was just changed via a password reset link.\n\nIf you didn't do this, someone else may have access to that account's email inbox.", username))
}

// ClientRegistered notifies every admin that a new display client
// registered and is waiting for approval.
func ClientRegistered(sqldb *sql.DB, clientName string) error {
	prefs, err := db.GetNotificationPreferences(sqldb)
	if err != nil {
		return err
	}
	return dispatch(sqldb, prefs.ClientRegistered, "client_registered", "New client pending approval",
		fmt.Sprintf("A new client %q registered on your Mullet server and is waiting for approval.\n\nApprove or reject it from the Clients page.", clientName))
}

// ClientApproved notifies every admin that a pending client was approved.
func ClientApproved(sqldb *sql.DB, clientName string) error {
	prefs, err := db.GetNotificationPreferences(sqldb)
	if err != nil {
		return err
	}
	return dispatch(sqldb, prefs.ClientApproved, "client_approved", "Client approved",
		fmt.Sprintf("The client %q was approved on your Mullet server and is now assigned to a display.", clientName))
}
