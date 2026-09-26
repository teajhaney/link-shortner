package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// save records
func (p *Postgres) Save(rec *URLRecord) error {
	ctx := context.Background()

	_, err := p.pool.Exec(ctx,
		`INSERT INTO urls (code, long_url, user_id, created_at, clicks)
		 VALUES ($1, $2, $3, $4, $5)`,
		rec.Code, rec.LongURL, rec.UserID, rec.CreatedAt, rec.Clicks,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrCodeConflict
	}
	return err
}

// get records
func (p *Postgres) Get(code string) (*URLRecord, error) {
	ctx := context.Background()

	var rec URLRecord
	// COALESCE maps the NULL owners of pre-account rows to an empty value,
	// so scanning never fails on legacy data.
	err := p.pool.QueryRow(ctx, `SELECT code, COALESCE(user_id::text, ''), long_url, created_at, clicks from urls WHERE code = $1`, code).Scan(&rec.Code, &rec.UserID, &rec.LongURL, &rec.CreatedAt, &rec.Clicks)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetAndIncrement looks a code up and records one click in a single atomic
// statement. Doing both in one query keeps the returned record in sync with the
// stored click count and avoids a race between the lookup and the increment.
func (p *Postgres) GetAndIncrement(code string) (*URLRecord, error) {
	ctx := context.Background()

	var rec URLRecord
	err := p.pool.QueryRow(ctx,
		`UPDATE urls SET clicks = clicks + 1 WHERE code = $1
		 RETURNING code, COALESCE(user_id::text, ''), long_url, created_at, clicks`,
		code,
	).Scan(&rec.Code, &rec.UserID, &rec.LongURL, &rec.CreatedAt, &rec.Clicks)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetByUser returns every link the user has shortened, newest first. The
// equality on user_id excludes both other users' rows and the NULL owners of
// pre-account rows, so the result is safe to return to the caller as-is.
func (p *Postgres) GetByUser(userID string) ([]URLRecord, error) {
	ctx := context.Background()

	rows, err := p.pool.Query(ctx,
		`SELECT code, COALESCE(user_id::text, ''), long_url, created_at, clicks
		 FROM urls WHERE user_id = $1::uuid ORDER BY created_at DESC, code`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]URLRecord, 0)
	for rows.Next() {
		var rec URLRecord
		if err := rows.Scan(&rec.Code, &rec.UserID, &rec.LongURL, &rec.CreatedAt, &rec.Clicks); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}
