package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Client is a registered dedicated client app (issue #29) -- see
// docs/architecture_1.md § Client for the full registration/polling/
// offline-mode design this backs.
type Client struct {
	ID          int
	ClientID    string
	Name        string
	DisplayID   *int
	Status      string // "pending" | "approved" | "rejected"
	LastSeenAt  *time.Time
	OfflineMode string
	Platform    *string
	AppVersion  *string
	IPAddress   *string
	CreatedAt   time.Time
}

const clientColumns = `id, client_id, name, display_id, status, last_seen_at, offline_mode, platform, app_version, ip_address, created_at`

func scanClient(row interface{ Scan(...any) error }) (Client, error) {
	var c Client
	if err := row.Scan(&c.ID, &c.ClientID, &c.Name, &c.DisplayID, &c.Status, &c.LastSeenAt, &c.OfflineMode, &c.Platform, &c.AppVersion, &c.IPAddress, &c.CreatedAt); err != nil {
		return Client{}, err
	}
	return c, nil
}

// RegisterClient records a new client pairing attempt, defaulting to
// status "pending" with no display assigned. Registration is
// idempotent on client_id: a client that re-sends the same
// registration (a lost response, or just re-registering defensively on
// every boot before it's confirmed pairing succeeded) gets back its
// existing record rather than an error, so retrying is always safe --
// but platform/app_version/ip_address are still refreshed on that
// repeat attempt, since any of the three can legitimately change
// between one registration and the next (an app update, a DHCP lease
// renewal) and there's no reason to keep serving stale values just
// because the client_id itself didn't change.
//
// The second return reports whether this was a genuinely new
// registration (as opposed to a repeat) -- issue #112's "new client
// pending approval" notification fires only on that first attempt, not
// on every retry a still-pending client makes before it's approved.
func RegisterClient(sqldb *sql.DB, clientID, name string, platform, appVersion, ipAddress *string) (Client, bool, error) {
	result, err := sqldb.Exec(
		`INSERT INTO clients (client_id, name, platform, app_version, ip_address) VALUES (?, ?, ?, ?, ?)`,
		clientID, name, platform, appVersion, ipAddress,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			if _, err := sqldb.Exec(
				`UPDATE clients SET platform = ?, app_version = ?, ip_address = ? WHERE client_id = ?`,
				platform, appVersion, ipAddress, clientID,
			); err != nil {
				return Client{}, false, fmt.Errorf("refreshing client %q: %w", clientID, err)
			}
			c, err := GetClientByClientID(sqldb, clientID)
			return c, false, err
		}
		return Client{}, false, fmt.Errorf("registering client %q: %w", clientID, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Client{}, false, fmt.Errorf("reading new client id: %w", err)
	}
	c, err := GetClient(sqldb, int(id))
	return c, true, err
}

// GetClient returns one client by its internal ID, or ErrNotFound.
func GetClient(sqldb *sql.DB, id int) (Client, error) {
	row := sqldb.QueryRow(`SELECT `+clientColumns+` FROM clients WHERE id = ?`, id)
	c, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	if err != nil {
		return Client{}, fmt.Errorf("getting client %d: %w", id, err)
	}
	return c, nil
}

// GetClientByClientID returns one client by its pairing client_id (the
// identifier the client itself generated and persists locally), or
// ErrNotFound.
func GetClientByClientID(sqldb *sql.DB, clientID string) (Client, error) {
	row := sqldb.QueryRow(`SELECT `+clientColumns+` FROM clients WHERE client_id = ?`, clientID)
	c, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	if err != nil {
		return Client{}, fmt.Errorf("getting client %q: %w", clientID, err)
	}
	return c, nil
}

// ListClients returns every registered client, oldest first.
func ListClients(sqldb *sql.DB) ([]Client, error) {
	rows, err := sqldb.Query(`SELECT ` + clientColumns + ` FROM clients ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning client: %w", err)
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// TouchClientLastSeen stamps last_seen_at to now and refreshes
// ip_address for a client's config poll. Returns ErrNotFound if
// clientID isn't registered, so the poller can tell its caller to
// re-register rather than polling forever against an ID the server has
// no record of.
func TouchClientLastSeen(sqldb *sql.DB, clientID, ipAddress string) error {
	result, err := sqldb.Exec(`UPDATE clients SET last_seen_at = CURRENT_TIMESTAMP, ip_address = ? WHERE client_id = ?`, ipAddress, clientID)
	if err != nil {
		return fmt.Errorf("touching client %q: %w", clientID, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking result for client %q: %w", clientID, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ApproveClient approves a pending (or previously rejected) client and
// assigns it to a display in one step -- the two are inseparable in
// practice, since an approved client with no display has nothing to
// render. Returns ErrNotFound if id doesn't exist, or ErrInUse if
// displayID doesn't exist.
func ApproveClient(sqldb *sql.DB, id, displayID int) error {
	result, err := sqldb.Exec(`UPDATE clients SET status = 'approved', display_id = ? WHERE id = ?`, displayID, id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("approving client %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// UpdateClient overwrites an editable client's name, display
// assignment (nil unassigns), status, and offline mode. Returns
// ErrNotFound if id doesn't exist, or ErrInUse if displayID is set but
// doesn't exist.
func UpdateClient(sqldb *sql.DB, id int, name string, displayID *int, status, offlineMode string) error {
	result, err := sqldb.Exec(
		`UPDATE clients SET name = ?, display_id = ?, status = ?, offline_mode = ? WHERE id = ?`,
		name, displayID, status, offlineMode, id,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("updating client %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// DeleteClient removes a client's registration entirely -- it would
// need to pair again from scratch to reconnect.
func DeleteClient(sqldb *sql.DB, id int) error {
	result, err := sqldb.Exec(`DELETE FROM clients WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting client %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// GetDisplayOfflineScreenHTML returns a display's admin-configured
// offline screen HTML, or nil if it hasn't set one (the caller falls
// back to a generated default). Returns ErrNotFound if displayID
// doesn't exist.
func GetDisplayOfflineScreenHTML(sqldb *sql.DB, displayID int) (*string, error) {
	var html sql.NullString
	err := sqldb.QueryRow(`SELECT offline_screen_html FROM displays WHERE id = ?`, displayID).Scan(&html)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting offline screen for display %d: %w", displayID, err)
	}
	if !html.Valid {
		return nil, nil
	}
	return &html.String, nil
}

// SetDisplayOfflineScreenHTML sets (or, with a nil html, clears back to
// the generated default) a display's custom offline screen HTML.
// Returns ErrNotFound if displayID doesn't exist.
func SetDisplayOfflineScreenHTML(sqldb *sql.DB, displayID int, html *string) error {
	result, err := sqldb.Exec(`UPDATE displays SET offline_screen_html = ? WHERE id = ?`, html, displayID)
	if err != nil {
		return fmt.Errorf("setting offline screen for display %d: %w", displayID, err)
	}
	return checkRowsAffected(result, displayID)
}
