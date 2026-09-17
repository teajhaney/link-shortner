package auth

import (
	"encoding/json"
	"io"
	"link-shortner/internal/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testUserID = "550e8400-e29b-41d4-a716-446655440000"

// fakeUserStore is the slice of database.Users that signin needs.
type fakeUserStore struct {
	rec *database.UserRecord
	err error
}

func (s *fakeUserStore) CreateUser(*database.UserRecord) error { return nil }

func (s *fakeUserStore) GetUserByEmail(string) (*database.UserRecord, error) { return s.rec, s.err }

func (s *fakeUserStore) GetUserByID(string) (*database.UserRecord, error) { return s.rec, s.err }

func (s *fakeUserStore) GetAllUsers() ([]database.UserRecord, error) { return nil, nil }

func (s *fakeUserStore) UpdateUser(string, database.UserUpdate) (*database.UserRecord, error) {
	return s.rec, s.err
}

func (s *fakeUserStore) DeleteUser(string) error { return nil }

// fakeRevokedStore is an in-memory revocation list.
type fakeRevokedStore struct {
	revoked map[string]time.Time
}

func (s *fakeRevokedStore) Revoke(tokenID, _ string, revokedAt time.Time) error {
	if s.revoked == nil {
		s.revoked = map[string]time.Time{}
	}
	if _, ok := s.revoked[tokenID]; !ok {
		s.revoked[tokenID] = revokedAt
	}
	return nil
}

func (s *fakeRevokedStore) IsRevoked(tokenID string) (bool, error) {
	_, ok := s.revoked[tokenID]
	return ok, nil
}

// testUser builds a user record with a real bcrypt hash for "password123".
func testUser(t *testing.T) *database.UserRecord {
	t.Helper()

	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	now := time.Now()
	return &database.UserRecord{
		ID:           testUserID,
		Name:         "Ada Lovelace",
		Email:        "ada@example.com",
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// authFixture wires the auth package the way main does, against fakes.
type authFixture struct {
	mux     *http.ServeMux
	tokens  *jwtService
	refresh *RefreshService
	store   *fakeRefreshStore
	revoked *fakeRevokedStore
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()

	tokens := newTestTokenService(t)
	store := newFakeRefreshStore()
	revoked := &fakeRevokedStore{}
	refresh := NewRefreshService(tokens, store)
	validator := NewTokenValidator(tokens, revoked)
	users := &fakeUserStore{rec: testUser(t)}

	mux := http.NewServeMux()
	NewHandler(NewSigninService(users, tokens, refresh), refresh, validator).RegisterRoutes(mux)

	return &authFixture{mux: mux, tokens: tokens, refresh: refresh, store: store, revoked: revoked}
}

func doRequest(t *testing.T, mux *http.ServeMux, method, target, body, accessToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, reader)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// decodeSession reads the token pair out of a signin or refresh response and
// checks the envelope a client relies on.
func decodeSession(t *testing.T, rec *httptest.ResponseRecorder) (access, refresh string) {
	t.Helper()

	var body struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body %q: %v", rec.Body.String(), err)
	}
	if body.TokenType != "Bearer" {
		t.Fatalf("token_type = %q, want Bearer", body.TokenType)
	}
	if want := int(tokenLifetime.Seconds()); body.ExpiresIn != want {
		t.Fatalf("expires_in = %d, want %d", body.ExpiresIn, want)
	}
	return body.Token, body.RefreshToken
}

// signIn is the shortest path to a live token pair in these tests.
func signIn(t *testing.T, fixture *authFixture) (access, refresh string) {
	t.Helper()

	rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/signin",
		`{"email":"ada@example.com","password":"password123"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("signin status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	return decodeSession(t, rec)
}

func TestSigninEndpointReturnsBothTokens(t *testing.T) {
	fixture := newAuthFixture(t)

	access, refresh := signIn(t, fixture)

	if access == "" || refresh == "" {
		t.Fatalf("signin returned access=%q refresh=%q, want both", access, refresh)
	}
	if _, err := fixture.tokens.Verify(access); err != nil {
		t.Fatalf("Verify(access) error = %v", err)
	}
	if fixture.store.active(testUserID) != 1 {
		t.Fatalf("active tokens = %d, want 1", fixture.store.active(testUserID))
	}
}

func TestSigninEndpointRejectsBadCredentials(t *testing.T) {
	fixture := newAuthFixture(t)

	rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/signin",
		`{"email":"ada@example.com","password":"nope"}`, "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if fixture.store.active(testUserID) != 0 {
		t.Fatal("a failed signin must not issue a refresh token")
	}
}

func TestRefreshEndpointRotatesTokens(t *testing.T) {
	fixture := newAuthFixture(t)
	_, firstRefresh := signIn(t, fixture)

	rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/refresh",
		`{"refresh_token":"`+firstRefresh+`"}`, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	access, secondRefresh := decodeSession(t, rec)
	if secondRefresh == firstRefresh {
		t.Fatal("refresh returned the same refresh token")
	}
	if _, err := fixture.tokens.Verify(access); err != nil {
		t.Fatalf("Verify(new access) error = %v", err)
	}
	if fixture.store.active(testUserID) != 1 {
		t.Fatalf("active tokens = %d, want 1 (rotated)", fixture.store.active(testUserID))
	}
}

func TestRefreshEndpointRejectsRotatedAwayToken(t *testing.T) {
	fixture := newAuthFixture(t)
	_, firstRefresh := signIn(t, fixture)

	first := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/refresh",
		`{"refresh_token":"`+firstRefresh+`"}`, "")
	if first.Code != http.StatusOK {
		t.Fatalf("first refresh status = %d, want %d", first.Code, http.StatusOK)
	}

	// Presenting the exchanged token again must fail, and must end every
	// session for that user as a precaution.
	replay := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/refresh",
		`{"refresh_token":"`+firstRefresh+`"}`, "")
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want %d", replay.Code, http.StatusUnauthorized)
	}
	if got := fixture.store.active(testUserID); got != 0 {
		t.Fatalf("active tokens = %d, want 0 after a replay", got)
	}
}

func TestRefreshEndpointRejectsUnknownToken(t *testing.T) {
	fixture := newAuthFixture(t)

	rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/refresh",
		`{"refresh_token":"neverissued"}`, "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogoutRevokesRefreshAndAccessTokens(t *testing.T) {
	fixture := newAuthFixture(t)
	access, refresh := signIn(t, fixture)

	rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/logout",
		`{"refresh_token":"`+refresh+`"}`, access)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fixture.store.active(testUserID) != 0 {
		t.Fatalf("active tokens = %d, want 0 after logout", fixture.store.active(testUserID))
	}

	// The access token's signature and expiry are still fine, so only the
	// revocation list can be rejecting it. This is the point of the check.
	verify := doRequest(t, fixture.mux, http.MethodGet, "/api/auth/verify", "", access)
	if verify.Code != http.StatusUnauthorized {
		t.Fatalf("verify after logout status = %d, want %d", verify.Code, http.StatusUnauthorized)
	}

	// And the refresh token cannot be exchanged any more.
	exchange := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/refresh",
		`{"refresh_token":"`+refresh+`"}`, "")
	if exchange.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status = %d, want %d", exchange.Code, http.StatusUnauthorized)
	}
}

func TestLogoutWorksWithoutAnAccessToken(t *testing.T) {
	fixture := newAuthFixture(t)
	_, refresh := signIn(t, fixture)

	// A client whose access token has expired must still be able to kill the
	// refresh token, so the body alone is a valid logout request.
	rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/logout",
		`{"refresh_token":"`+refresh+`"}`, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fixture.store.active(testUserID) != 0 {
		t.Fatalf("active tokens = %d, want 0", fixture.store.active(testUserID))
	}
}

func TestLogoutIsIdempotentAndNeedsACredential(t *testing.T) {
	fixture := newAuthFixture(t)

	// Nothing to revoke is a client error, not a silent success.
	if rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/logout", "", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty logout status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	_, refresh := signIn(t, fixture)
	body := `{"refresh_token":"` + refresh + `"}`

	for i := 1; i <= 2; i++ {
		if rec := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/logout", body, ""); rec.Code != http.StatusOK {
			t.Fatalf("logout #%d status = %d, want %d", i, rec.Code, http.StatusOK)
		}
	}
}

func TestMiddlewareRejectsRevokedAccessToken(t *testing.T) {
	fixture := newAuthFixture(t)
	validator := NewTokenValidator(fixture.tokens, fixture.revoked)

	var reached bool
	protected := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	access, refresh := signIn(t, fixture)

	// Before logout the wrapped handler runs.
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !reached {
		t.Fatalf("status before revoking = %d (reached=%v), want %d", rec.Code, reached, http.StatusOK)
	}

	// Log out both halves, then present the same access token again. Its
	// signature and expiry are still valid, so only the revocation list can
	// be rejecting it.
	logout := doRequest(t, fixture.mux, http.MethodPost, "/api/auth/logout",
		`{"refresh_token":"`+refresh+`"}`, access)
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d", logout.Code, http.StatusOK)
	}

	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status after revoking = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsMissingHeader(t *testing.T) {
	fixture := newAuthFixture(t)
	protected := Middleware(NewTokenValidator(fixture.tokens, fixture.revoked))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAccessTokenLifetimeIsShorterThanRefreshToken(t *testing.T) {
	if tokenLifetime > time.Hour {
		t.Fatalf("tokenLifetime = %v, want an access token measured in minutes", tokenLifetime)
	}
	if refreshTokenLifetime <= tokenLifetime {
		t.Fatalf("refreshTokenLifetime = %v must outlive the access token (%v)", refreshTokenLifetime, tokenLifetime)
	}
}

// The database looks a refresh token up by hash, so the hash must be stable and
// must not be the token itself.
func TestHashRefreshTokenIsStableAndOneWay(t *testing.T) {
	const token = "some-refresh-token"

	if HashRefreshToken(token) != HashRefreshToken(token) {
		t.Fatal("HashRefreshToken() is not deterministic")
	}
	if HashRefreshToken(token) == token {
		t.Fatal("HashRefreshToken() returned the token unchanged")
	}
	if got := len(HashRefreshToken(token)); got != 64 {
		t.Fatalf("hash length = %d, want 64 hex characters", got)
	}
	if HashRefreshToken(token) == HashRefreshToken(token+"x") {
		t.Fatal("different tokens produced the same hash")
	}
}
