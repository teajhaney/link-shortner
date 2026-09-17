package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"link-shortner/internal/database"
	"link-shortner/internal/shortcode"
)

// ErrInvalidRefreshToken covers every reason a refresh token cannot be
// exchanged: unknown, revoked, replayed, or expired. They are one error on
// purpose, because the client's only correct response is the same in all
// cases: sign in again.
var ErrInvalidRefreshToken = errors.New("invalid refresh token")

const (
	// refreshTokenLifetime is how long a client can keep exchanging a refresh
	// token. It is far longer than the access token: that one is short lived
	// to limit the damage of a leak, while this one is what keeps a user
	// signed in without re-entering a password.
	refreshTokenLifetime = 30 * 24 * time.Hour

	// refreshTokenBytes is the entropy of the token before encoding. 48 bytes
	// of Base62 is far beyond guessing range, which is why the stored hash
	// needs no salt: unlike a password, it cannot be brute forced.
	refreshTokenBytes = 48
)

// Session is a matched pair of tokens: a short lived access token, and the
// refresh token that replaces it.
type Session struct {
	AccessToken      string
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// RefreshService issues and rotates refresh tokens. It holds no mutable state
// after New, so it is safe for concurrent use.
type RefreshService struct {
	tokens *jwtService
	store  database.RefreshTokens
	// now is injectable so tests can exercise expiry without waiting.
	now func() time.Time
}

// NewRefreshService wires the JWT service to a refresh-token store. The store
// must be the same Postgres pool the rest of the app uses, so a token rotated
// by one instance is seen as used by all of them.
func NewRefreshService(tokens *jwtService, store database.RefreshTokens) *RefreshService {
	return &RefreshService{tokens: tokens, store: store, now: time.Now}
}

// Issue mints a refresh token for a user and returns the plaintext plus its
// expiry. This is the only moment the plaintext exists outside the client;
// what gets stored is the hash.
func (s *RefreshService) Issue(userID string) (string, time.Time, error) {
	token, err := shortcode.RandomBase62(refreshTokenBytes)
	if err != nil {
		return "", time.Time{}, err
	}

	now := s.now()
	expiresAt := now.Add(refreshTokenLifetime)
	rec := &database.RefreshTokenRecord{
		UserID:    userID,
		TokenHash: HashRefreshToken(token),
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	if err := s.store.SaveRefreshToken(rec); err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

// Refresh exchanges a refresh token for a new pair, rotating the token so the
// one presented can never be used again.
//
// Rotation is what makes replay detectable. A token that was already exchanged
// and shows up again means either the client replayed it or someone stole it,
// so every token for that user is revoked rather than just rejecting the one.
func (s *RefreshService) Refresh(token string) (*Session, error) {
	if token == "" {
		return nil, ErrInvalidRefreshToken
	}

	rec, err := s.store.GetRefreshToken(HashRefreshToken(token))
	if err != nil {
		// An unknown hash is deliberately indistinguishable from a token that
		// was never issued.
		if errors.Is(err, database.ErrRefreshTokenNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	now := s.now()

	if rec.RevokedAt != nil {
		if err := s.store.RevokeAllRefreshTokens(rec.UserID, now); err != nil {
			return nil, err
		}
		return nil, ErrInvalidRefreshToken
	}
	if now.After(rec.ExpiresAt) {
		// Expired but still on file: retire it so it stops being a candidate,
		// then report the failure.
		if err := s.store.RevokeRefreshToken(rec.TokenHash, now); err != nil {
			return nil, err
		}
		return nil, ErrInvalidRefreshToken
	}

	access, err := s.tokens.Issue(rec.UserID)
	if err != nil {
		return nil, err
	}

	// Revoke before issuing the replacement. If the process dies in between,
	// the client is simply logged out, which is the safe direction; the
	// opposite order could leave two usable refresh tokens.
	if err := s.store.RevokeRefreshToken(rec.TokenHash, now); err != nil {
		return nil, err
	}
	refresh, refreshExpiresAt, err := s.Issue(rec.UserID)
	if err != nil {
		return nil, err
	}

	return &Session{
		AccessToken:      access,
		RefreshToken:     refresh,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// Revoke invalidates a single refresh token. It reports no error for a token
// that is unknown or already revoked, so logout can be sent twice and still
// succeed, and a client cannot probe for which tokens exist.
func (s *RefreshService) Revoke(token string) error {
	if token == "" {
		return nil
	}

	err := s.store.RevokeRefreshToken(HashRefreshToken(token), s.now())
	if errors.Is(err, database.ErrRefreshTokenNotFound) {
		return nil
	}
	return err
}

// RevokeAll invalidates every refresh token for a user. It backs "sign out
// everywhere" and the compromise response, and is what a password change
// should call.
func (s *RefreshService) RevokeAll(userID string) error {
	return s.store.RevokeAllRefreshTokens(userID, s.now())
}

// HashRefreshToken returns the hex SHA-256 of a refresh token, which is what
// the database stores. SHA-256 with no salt is the right choice here: the
// token is 48 random bytes, so there is no dictionary to attack.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
