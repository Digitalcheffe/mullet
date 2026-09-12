package db

import (
	"errors"
	"testing"
)

func TestUserEmailAndPasswordRoundTrip(t *testing.T) {
	sqldb := newTestDB(t)

	result, err := sqldb.Exec(`INSERT INTO users (username, password_hash) VALUES ('admin', 'hash1')`)
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	id64, _ := result.LastInsertId()
	id := int(id64)

	got, err := GetUser(sqldb, id)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Username != "admin" || got.Email != nil {
		t.Errorf("GetUser = %+v, want email nil before it's set", got)
	}

	byUsername, err := GetUserByUsername(sqldb, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if byUsername.ID != id {
		t.Errorf("GetUserByUsername.ID = %d, want %d", byUsername.ID, id)
	}

	if err := SetUserEmail(sqldb, id, "admin@example.com"); err != nil {
		t.Fatalf("SetUserEmail: %v", err)
	}
	got, _ = GetUser(sqldb, id)
	if got.Email == nil || *got.Email != "admin@example.com" {
		t.Errorf("GetUser.Email = %v, want admin@example.com", got.Email)
	}

	// Clearing it back to empty should null it out, not store "".
	if err := SetUserEmail(sqldb, id, ""); err != nil {
		t.Fatalf("SetUserEmail (clear): %v", err)
	}
	got, _ = GetUser(sqldb, id)
	if got.Email != nil {
		t.Errorf("GetUser.Email = %v, want nil after clearing", got.Email)
	}

	if err := SetUserPasswordHash(sqldb, id, "hash2"); err != nil {
		t.Fatalf("SetUserPasswordHash: %v", err)
	}
	got, _ = GetUser(sqldb, id)
	if got.PasswordHash != "hash2" {
		t.Errorf("GetUser.PasswordHash = %q, want hash2", got.PasswordHash)
	}
}

func TestGetUserMissingReturnsErrNotFound(t *testing.T) {
	sqldb := newTestDB(t)

	if _, err := GetUser(sqldb, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUser(missing) = %v, want ErrNotFound", err)
	}
	if _, err := GetUserByUsername(sqldb, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUserByUsername(missing) = %v, want ErrNotFound", err)
	}
	if err := SetUserEmail(sqldb, 9999, "x@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetUserEmail(missing) = %v, want ErrNotFound", err)
	}
	if err := SetUserPasswordHash(sqldb, 9999, "hash"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetUserPasswordHash(missing) = %v, want ErrNotFound", err)
	}
}

func TestCreateAndListUsers(t *testing.T) {
	sqldb := newTestDB(t)

	if count, err := CountUsers(sqldb); err != nil || count != 0 {
		t.Fatalf("CountUsers (empty) = %d, %v, want 0, nil", count, err)
	}

	first, err := CreateUser(sqldb, "alice", "hash1")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if first.Username != "alice" || first.ID == 0 {
		t.Errorf("CreateUser = %+v, want a real id and username alice", first)
	}

	if _, err := CreateUser(sqldb, "bob", "hash2"); err != nil {
		t.Fatalf("CreateUser (second): %v", err)
	}

	if count, err := CountUsers(sqldb); err != nil || count != 2 {
		t.Fatalf("CountUsers = %d, %v, want 2, nil", count, err)
	}

	users, err := ListUsers(sqldb)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 || users[0].Username != "alice" || users[1].Username != "bob" {
		t.Errorf("ListUsers = %+v, want [alice, bob] in creation order", users)
	}
}

func TestCreateUserDuplicateUsernameReturnsErrUsernameTaken(t *testing.T) {
	sqldb := newTestDB(t)

	if _, err := CreateUser(sqldb, "alice", "hash1"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := CreateUser(sqldb, "alice", "hash2"); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("CreateUser (duplicate) = %v, want ErrUsernameTaken", err)
	}
}

func TestDeleteUser(t *testing.T) {
	sqldb := newTestDB(t)

	alice, err := CreateUser(sqldb, "alice", "hash1")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bob, err := CreateUser(sqldb, "bob", "hash2")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Refuses to remove the last account -- with two, either can go.
	if err := DeleteUser(sqldb, alice.ID); err != nil {
		t.Fatalf("DeleteUser (alice, two remain): %v", err)
	}

	if err := DeleteUser(sqldb, bob.ID); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("DeleteUser (bob, last one) = %v, want ErrLastAdmin", err)
	}

	if count, err := CountUsers(sqldb); err != nil || count != 1 {
		t.Fatalf("CountUsers after refused delete = %d, %v, want 1, nil (bob must still exist)", count, err)
	}

	if err := DeleteUser(sqldb, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteUser(missing) = %v, want ErrNotFound", err)
	}
}
