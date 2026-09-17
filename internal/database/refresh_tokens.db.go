package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SaveRefreshToken stores a newly issued refresh token. Only the hash is kept,
// so the plaintext never reaches the database.
func (p *Postgres) SaveRefreshToken(rec *RefreshTokenRecord) error {
	ctx := context.Background()

	_, err := p.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, $4)`,
		rec.UserID, rec.TokenHash, rec.ExpiresAt, rec.CreatedAt,
	)
	return err
}

// GetRefreshToken looks a token up by hash. Revoked and expired tokens are
// returned rather than filtered out, because the caller needs to tell "never
// issued" apart from "already used and presented again".
func (p *Postgres) GetRefreshToken(tokenHash string) (*RefreshTokenRecord, error) {
	ctx := context.Background()

	var rec RefreshTokenRecord
	err := p.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		 FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&rec.ID, &rec.UserID, &rec.TokenHash, &rec.ExpiresAt, &rec.RevokedAt, &rec.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// RevokeRefreshToken marks a token revoked while keeping the first revocation
// time, so replay detection still reports when the token was retired. An
// unknown hash is ErrRefreshTokenNotFound; an already revoked one succeeds, so
// logout stays idempotent.
func (p *Postgres) RevokeRefreshToken(tokenHash string, revokedAt time.Time) error {
	ctx := context.Background()

	tag, err := p.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2) WHERE token_hash = $1`,
		tokenHash, revokedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRefreshTokenNotFound
	}
	return nil
}

// RevokeAllRefreshTokens invalidates every usable token for a user. It is the
// response to a replayed token, and what a password change should call.
func (p *Postgres) RevokeAllRefreshTokens(userID string, revokedAt time.Time) error {
	ctx := context.Background()

	// The "revoked_at IS NULL" filter keeps already recorded revocation times
	// intact rather than stamping them all with the latest one.
	_, err := p.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`,
		userID, revokedAt,
	)
	return err
}
