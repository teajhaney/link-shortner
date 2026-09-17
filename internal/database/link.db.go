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
		`INSERT INTO urls (code, long_url, created_at, clicks)
		 VALUES ($1, $2, $3, $4)`,
		rec.Code, rec.LongURL, rec.CreatedAt, rec.Clicks,
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
	err := p.pool.QueryRow(ctx, `SELECT code, long_url, created_at, clicks from urls WHERE code = $1`, code).Scan(&rec.Code, &rec.LongURL, &rec.CreatedAt, &rec.Clicks)

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
		 RETURNING code, long_url, created_at, clicks`,
		code,
	).Scan(&rec.Code, &rec.LongURL, &rec.CreatedAt, &rec.Clicks)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}
