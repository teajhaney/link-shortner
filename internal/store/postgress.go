package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres open a connection pool to the given DSN
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)

	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

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

// click counts
func (p *Postgres) IncrementClicks(code string) error {
	ctx := context.Background()

	tag, err := p.pool.Exec(ctx,
		`UPDATE urls SET clicks = clicks + 1 WHERE code = $1`,
		code,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// NextID pulls the next value from Postgres's own auto-increment
// sequence for the urls table, without inserting a row yet. BIGSERIAL
// columns create this sequence automatically, named "<table>_<column>_seq".
func (p *Postgres) NextID() uint64 {
	ctx := context.Background()

	var id uint64
	// A failure here is rare (would mean the DB is unreachable) and
	// NextID's signature has no error return, matching the in-memory
	// version's signature -- in real production code you'd likely
	// change the Store interface to return an error here too.
	_ = p.pool.QueryRow(ctx, `SELECT nextval('urls_id_seq')`).Scan(&id)
	return id
}
