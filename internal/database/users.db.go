package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// UpdateUser updates only the supplied fields and returns safe user data.
func (p *Postgres) UpdateUser(id string, update UserUpdate) (*UserRecord, error) {
	ctx := context.Background()

	setClauses := make([]string, 0, 4)
	args := make([]any, 0, 5)
	if update.Name != nil {
		args = append(args, *update.Name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", len(args)))
	}
	if update.Email != nil {
		args = append(args, *update.Email)
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", len(args)))
	}
	if update.PasswordHash != nil {
		args = append(args, *update.PasswordHash)
		setClauses = append(setClauses, fmt.Sprintf("password_hash = $%d", len(args)))
	}
	args = append(args, update.UpdatedAt)
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", len(args)))
	args = append(args, id)

	query := fmt.Sprintf(
		"UPDATE users SET %s WHERE id = $%d RETURNING id, name, email, created_at, updated_at",
		strings.Join(setClauses, ", "),
		len(args),
	)

	var rec UserRecord
	err := p.pool.QueryRow(ctx, query, args...).Scan(&rec.ID, &rec.Name, &rec.Email, &rec.CreatedAt, &rec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, ErrEmailConflict
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (p *Postgres) DeleteUser(id string) error {
	ctx := context.Background()

	tag, err := p.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
