package database

import (
	"errors"
	"time"
)

var (
	ErrNotFound             = errors.New("short code not found")
	ErrCodeConflict         = errors.New("short code already exists")
	ErrEmailConflict        = errors.New("email already exists")
	ErrUserNotFound         = errors.New("user not found")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
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
	// GetAndIncrement resolves a code while recording exactly one click. It
	// returns the updated record, or ErrNotFound when the code is unknown.
	GetAndIncrement(code string) (*URLRecord, error)
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

type UserUpdate struct {
	Name         *string
	Email        *string
	PasswordHash *string
	UpdatedAt    time.Time
}

// user Store
type Users interface {
	CreateUser(rec *UserRecord) error
	GetUserByEmail(email string) (*UserRecord, error)
	GetUserByID(id string) (*UserRecord, error)
	GetAllUsers() ([]UserRecord, error)
	UpdateUser(id string, update UserUpdate) (*UserRecord, error)
	DeleteUser(id string) error
}

// RevokedTokens is the revocation list. Implementations record token IDs that
// must no longer be accepted, even though their signature and expiry are still
// valid.
type RevokedTokens interface {
	Revoke(tokenID, userID string, revokedAt time.Time) error
	IsRevoked(tokenID string) (bool, error)
}

// RefreshTokenRecord is a stored refresh token. Only the hash is persisted, so
// this record is never enough to impersonate a client.
type RefreshTokenRecord struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	// RevokedAt is nil while the token is usable. A non-nil value on a token
	// that is presented for exchange means the token was replayed.
	RevokedAt *time.Time
	CreatedAt time.Time
}

// RefreshTokens is the refresh-token store. These tokens are long lived, so
// they are rotated on every use and must be revocable individually and per
// user.
type RefreshTokens interface {
	SaveRefreshToken(rec *RefreshTokenRecord) error
	// GetRefreshToken returns revoked and expired tokens too: the caller has
	// to tell "never issued" apart from "replayed".
	GetRefreshToken(tokenHash string) (*RefreshTokenRecord, error)
	RevokeRefreshToken(tokenHash string, revokedAt time.Time) error
	RevokeAllRefreshTokens(userID string, revokedAt time.Time) error
}
