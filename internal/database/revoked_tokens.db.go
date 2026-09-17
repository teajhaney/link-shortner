package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Revoke records that a token ID is no longer valid. It is idempotent: revoking
// the same ID twice neither fails nor duplicates the row.
func (p *Postgres) Revoke(tokenID, userID string, revokedAt time.Time) error {
	ctx := context.Background()

	// ON CONFLICT DO NOTHING makes this safe to call repeatedly, so a client
	// that sends logout twice gets a clean 200 both times.
	const query = `
		INSERT INTO revoked_tokens (token_id, user_id, revoked_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_id) DO NOTHING
	`
	_, err := p.pool.Exec(ctx, query, tokenID, userID, revokedAt)
	return err
}

// IsRevoked reports whether a token ID has been revoked. A token that was
// never issued, or was issued after this record was written, is not revoked.
func (p *Postgres) IsRevoked(tokenID string) (bool, error) {
	ctx := context.Background()

	var revokedAt time.Time
	err := p.pool.QueryRow(ctx, `SELECT revoked_at FROM revoked_tokens WHERE token_id = $1`, tokenID).Scan(&revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
