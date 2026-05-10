package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/akinalp/mqvi/pkg"
	_ "modernc.org/sqlite"
)

func newUserRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		PRAGMA foreign_keys=ON;
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			token_version INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			refresh_token TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL
		);
		CREATE TABLE password_reset_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestSQLiteUserRepo_UpdatePasswordConflictDoesNotRotate(t *testing.T) {
	ctx := context.Background()
	db := newUserRepoTestDB(t)
	repo := NewSQLiteUserRepo(db)
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, username, password_hash, token_version) VALUES ('u1', 'u1', 'old-hash', 0)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err := repo.UpdatePassword(ctx, "u1", "stale-hash", "new-hash")
	if !errors.Is(err, pkg.ErrConflict) {
		t.Fatalf("got %v want ErrConflict", err)
	}

	var passwordHash string
	var tokenVersion int
	if err := db.QueryRowContext(ctx, `SELECT password_hash, token_version FROM users WHERE id = 'u1'`).Scan(&passwordHash, &tokenVersion); err != nil {
		t.Fatalf("select user: %v", err)
	}
	if passwordHash != "old-hash" || tokenVersion != 0 {
		t.Fatalf("got password_hash=%q token_version=%d", passwordHash, tokenVersion)
	}
}

func TestSQLiteUserRepo_ResetPasswordWithTokenClearsSiblingTokensAndSessions(t *testing.T) {
	ctx := context.Background()
	db := newUserRepoTestDB(t)
	repo := NewSQLiteUserRepo(db)
	expiresAt := time.Now().Add(time.Hour)
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, username, password_hash, token_version) VALUES ('u1', 'u1', 'old-hash', 0)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (id, user_id, refresh_token, expires_at) VALUES ('s1', 'u1', 'r1', ?)`, expiresAt); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at) VALUES ('rt1', 'u1', 'h1', ?), ('rt2', 'u1', 'h2', ?)`, expiresAt, expiresAt); err != nil {
		t.Fatalf("insert reset tokens: %v", err)
	}

	newTV, err := repo.ResetPasswordWithToken(ctx, "u1", "rt1", "new-hash")
	if err != nil {
		t.Fatalf("ResetPasswordWithToken: %v", err)
	}
	if newTV != 1 {
		t.Fatalf("newTV got %d want 1", newTV)
	}

	var passwordHash string
	var tokenVersion int
	if err := db.QueryRowContext(ctx, `SELECT password_hash, token_version FROM users WHERE id = 'u1'`).Scan(&passwordHash, &tokenVersion); err != nil {
		t.Fatalf("select user: %v", err)
	}
	if passwordHash != "new-hash" || tokenVersion != 1 {
		t.Fatalf("got password_hash=%q token_version=%d", passwordHash, tokenVersion)
	}

	var resetCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = 'u1'`).Scan(&resetCount); err != nil {
		t.Fatalf("count reset tokens: %v", err)
	}
	if resetCount != 0 {
		t.Fatalf("reset token count got %d want 0", resetCount)
	}

	var sessionCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id = 'u1'`).Scan(&sessionCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 0 {
		t.Fatalf("session count got %d want 0", sessionCount)
	}
}
