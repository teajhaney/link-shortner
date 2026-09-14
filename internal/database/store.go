package database

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("short code not found")
	ErrCodeConflict  = errors.New("short code already exists")
	ErrEmailConflict = errors.New("email already exists")
	ErrUserNotFound  = errors.New("user not found")
)

// records for everything tracked by the system for a given shortlinked
type URLRecord struct {
	Code      string
	LongURL   string
	CreatedAt time.Time
	Clicks    int64
}

// Store is the storage contract. Anything satisfying this interface
// (in-memory, Redis, Postgres, ...) can back the shortener without
// the HTTP handlers needing to change.
type Link interface {
	Save(rec *URLRecord) error
	Get(code string) (*URLRecord, error)
	IncrementClicks(code string) error
}

// records for user
type UserRecord struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// user Store
type Users interface {
	CreateUser(rec *UserRecord) error
	GetUserByEmail(email string) (*UserRecord, error)
	GetUserByID(id string) (*UserRecord, error)
}
