package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// create user
func (p *Postgres) CreateUser(rec *UserRecord) error {
	ctx := context.Background()

	_, err := p.pool.Exec(ctx, `INSERT INTO users (name, email, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`, rec.Name, rec.Email, rec.PasswordHash, rec.CreatedAt, rec.UpdatedAt)

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailConflict
	}
	return err
}

// get user by email
func (p *Postgres) GetUserByEmail(email string) (*UserRecord, error) {
	ctx := context.Background()

	var rec UserRecord
	err := p.pool.QueryRow(ctx, `SELECT id, name, email, password_hash, created_at, updated_at FROM users WHERE email = $1`, email).Scan(&rec.ID, &rec.Name, &rec.Email, &rec.PasswordHash, &rec.CreatedAt, &rec.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// get user by ID
func (p *Postgres) GetUserByID(id string) (*UserRecord, error) {
	ctx := context.Background()

	var rec UserRecord
	err := p.pool.QueryRow(ctx, `SELECT id, name, email, created_at, updated_at FROM users WHERE id = $1`, id).Scan(&rec.ID, &rec.Name, &rec.Email, &rec.CreatedAt, &rec.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetAllUsers returns all users ordered by creation time.
func (p *Postgres) GetAllUsers() ([]UserRecord, error) {
	ctx := context.Background()

	var recs []UserRecord
	rows, err := p.pool.Query(ctx, `SELECT id, name, email, created_at, updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rec UserRecord
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Email, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return recs, nil
}
