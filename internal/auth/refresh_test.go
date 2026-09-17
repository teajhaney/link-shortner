package auth

import (
	"errors"
	"testing"
	"time"

	"link-shortner/internal/database"
)

// fakeRefreshStore is an in-memory refresh-token store. It records revocation
// times the way Postgres does: the first revocation wins, so a replay can still
// be told apart from a first use.
type fakeRefreshStore struct {
	byHash map[string]*database.RefreshTokenRecord
}

func newFakeRefreshStore() *fakeRefreshStore {
	return &fakeRefreshStore{byHash: map[string]*database.RefreshTokenRecord{}}
}

func (s *fakeRefreshStore) SaveRefreshToken(rec *database.RefreshTokenRecord) error {
	clone := *rec
	s.byHash[rec.TokenHash] = &clone
	return nil
}

func (s *fakeRefreshStore) GetRefreshToken(tokenHash string) (*database.RefreshTokenRecord, error) {
	rec, ok := s.byHash[tokenHash]
	if !ok {
		return nil, database.ErrRefreshTokenNotFound
	}
	clone := *rec
	return &clone, nil
}

func (s *fakeRefreshStore) RevokeRefreshToken(tokenHash string, revokedAt time.Time) error {
	rec, ok := s.byHash[tokenHash]
	if !ok {
		return database.ErrRefreshTokenNotFound
	}
	if rec.RevokedAt == nil {
		at := revokedAt
		rec.RevokedAt = &at
	}
	return nil
}

func (s *fakeRefreshStore) RevokeAllRefreshTokens(userID string, revokedAt time.Time) error {
	for _, rec := range s.byHash {
		if rec.UserID == userID && rec.RevokedAt == nil {
			at := revokedAt
			rec.RevokedAt = &at
		}
	}
	return nil
}

// active reports how many usable tokens a user still has.
func (s *fakeRefreshStore) active(userID string) int {
	count := 0
	for _, rec := range s.byHash {
		if rec.UserID == userID && rec.RevokedAt == nil {
			count++
		}
	}
	return count
}

func TestIssueStoresOnlyTheHash(t *testing.T) {
	store := newFakeRefreshStore()
	service := NewRefreshService(newTestTokenService(t), store)

	token, expiresAt, err := service.Issue(testUserID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token == "" {
		t.Fatal("Issue() returned an empty token")
	}
	if len(store.byHash) != 1 {
		t.Fatalf("stored %d records, want 1", len(store.byHash))
	}

	rec := store.byHash[HashRefreshToken(token)]
	if rec == nil {
		t.Fatal("Issue() did not store the token under its hash")
	}
	if rec.TokenHash == token {
		t.Fatal("Issue() stored the plaintext token")
	}
	if rec.TokenHash != HashRefreshToken(token) {
		t.Fatalf("TokenHash = %q, want the SHA-256 of the token", rec.TokenHash)
	}
	if got := expiresAt.Sub(rec.CreatedAt); got != refreshTokenLifetime {
		t.Fatalf("lifetime = %v, want %v", got, refreshTokenLifetime)
	}
}

func TestRefreshRotatesTheToken(t *testing.T) {
	store := newFakeRefreshStore()
	tokens := newTestTokenService(t)
	service := NewRefreshService(tokens, store)
	userID := testUserID

	original, _, err := service.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	session, err := service.Refresh(original)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if session.RefreshToken == original {
		t.Fatal("Refresh() returned the same refresh token, want a rotated one")
	}
	if store.active(userID) != 1 {
		t.Fatalf("active tokens = %d, want 1 (old revoked, new live)", store.active(userID))
	}

	// The new access token must be usable and belong to the same user.
	claims, err := tokens.Verify(session.AccessToken)
	if err != nil {
		t.Fatalf("Verify(new access token) error = %v", err)
	}
	if claims.UserID != userID {
		t.Fatalf("claims.UserID = %q, want %q", claims.UserID, userID)
	}

	// And the one that was just exchanged must not work again.
	if _, err := service.Refresh(original); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Refresh(replayed) error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRefreshTreatsReplayAsCompromise(t *testing.T) {
	store := newFakeRefreshStore()
	service := NewRefreshService(newTestTokenService(t), store)
	userID := testUserID

	first, _, err := service.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, _, err := service.Issue(userID); err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := service.Refresh(first); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	// Replaying an already-exchanged token is no longer just one bad token,
	// it is evidence the token leaked.
	if _, err := service.Refresh(first); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Refresh(replayed) error = %v, want ErrInvalidRefreshToken", err)
	}

	if got := store.active(userID); got != 0 {
		t.Fatalf("active tokens = %d, want 0 after a replay was detected", got)
	}
}

func TestRefreshRejectsUnknownAndEmptyTokens(t *testing.T) {
	service := NewRefreshService(newTestTokenService(t), newFakeRefreshStore())

	for _, token := range []string{"", "neverissued"} {
		if _, err := service.Refresh(token); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("Refresh(%q) error = %v, want ErrInvalidRefreshToken", token, err)
		}
	}
}

func TestRefreshRejectsExpiredToken(t *testing.T) {
	store := newFakeRefreshStore()
	service := NewRefreshService(newTestTokenService(t), store)
	userID := testUserID

	token, _, err := service.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	// Jump past the expiry instead of waiting it out.
	service.now = func() time.Time { return time.Now().Add(refreshTokenLifetime + time.Minute) }

	if _, err := service.Refresh(token); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Refresh(expired) error = %v, want ErrInvalidRefreshToken", err)
	}
	if got := store.active(userID); got != 0 {
		t.Fatalf("active tokens = %d, want 0 so the expired one is retired", got)
	}
}

func TestRevokeIsIdempotent(t *testing.T) {
	store := newFakeRefreshStore()
	service := NewRefreshService(newTestTokenService(t), store)

	token, _, err := service.Issue(testUserID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if err := service.Revoke(token); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	// Signing out twice must succeed, and an unknown token must not reveal
	// itself by failing.
	if err := service.Revoke(token); err != nil {
		t.Fatalf("Revoke() twice error = %v", err)
	}
	if err := service.Revoke("neverissued"); err != nil {
		t.Fatalf("Revoke(unknown) error = %v", err)
	}
	if err := service.Revoke(""); err != nil {
		t.Fatalf("Revoke(empty) error = %v", err)
	}

	if got := store.active(testUserID); got != 0 {
		t.Fatalf("active tokens = %d, want 0", got)
	}
}

func TestRevokeAllEndsEverySessionForAUser(t *testing.T) {
	store := newFakeRefreshStore()
	service := NewRefreshService(newTestTokenService(t), store)
	const otherUser = "11111111-1111-1111-1111-111111111111"

	for i := 0; i < 3; i++ {
		if _, _, err := service.Issue(testUserID); err != nil {
			t.Fatalf("Issue() error = %v", err)
		}
	}
	if _, _, err := service.Issue(otherUser); err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if err := service.RevokeAll(testUserID); err != nil {
		t.Fatalf("RevokeAll() error = %v", err)
	}

	if got := store.active(testUserID); got != 0 {
		t.Fatalf("active tokens for the user = %d, want 0", got)
	}
	if got := store.active(otherUser); got != 1 {
		t.Fatalf("active tokens for the other user = %d, want 1 (must not be touched)", got)
	}
}

func TestSigninReturnsBothTokens(t *testing.T) {
	store := newFakeRefreshStore()
	tokens := newTestTokenService(t)
	refresh := NewRefreshService(tokens, store)
	signin := NewSigninService(&fakeUserStore{rec: testUser(t)}, tokens, refresh)

	session, err := signin.Signin("ada@example.com", "password123")
	if err != nil {
		t.Fatalf("Signin() error = %v", err)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatalf("Signin() session = %#v, want both tokens", session)
	}

	claims, err := tokens.Verify(session.AccessToken)
	if err != nil {
		t.Fatalf("Verify(access) error = %v", err)
	}
	if claims.UserID != testUserID {
		t.Fatalf("claims.UserID = %q, want %q", claims.UserID, testUserID)
	}
	if store.active(testUserID) != 1 {
		t.Fatalf("active tokens = %d, want the refresh token signin handed back", store.active(testUserID))
	}

	// Wrong credentials must not start a session.
	if _, err := signin.Signin("ada@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Signin(wrong password) error = %v, want ErrInvalidCredentials", err)
	}
}
