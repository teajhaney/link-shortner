package users

import (
	"encoding/json"
	"io"
	"link-shortner/internal/auth"
	"link-shortner/internal/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testUserID = "550e8400-e29b-41d4-a716-446655440000"

// errorStore returns preconfigured results so handler error mapping can be
// exercised without a database. It also records the arguments it received.
type errorStore struct {
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	record    *database.UserRecord
	seenEmail string
	seenID    string
}

type staticVerifier struct {
	userID string
}

func (v *staticVerifier) Verify(string) (*auth.Claims, error) {
	return &auth.Claims{UserID: v.userID}, nil
}

func (s *errorStore) CreateUser(*database.UserRecord) error { return s.createErr }

func (s *errorStore) GetUserByEmail(email string) (*database.UserRecord, error) {
	s.seenEmail = email
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.record, nil
}

func (s *errorStore) GetUserByID(id string) (*database.UserRecord, error) {
	s.seenID = id
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.record, nil
}

func (s *errorStore) GetAllUsers() ([]database.UserRecord, error) { return nil, nil }

func (s *errorStore) UpdateUser(string, database.UserUpdate) (*database.UserRecord, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return s.record, nil
}

func (s *errorStore) DeleteUser(string) error { return s.deleteErr }

func newTestMuxWithUser(store database.Users, userID string) *http.ServeMux {
	mux := http.NewServeMux()
	NewHandler(NewService(store), auth.Middleware(&staticVerifier{userID: userID})).RegisterRoutes(mux)
	return mux
}

func newTestMux(store database.Users) *http.ServeMux {
	return newTestMuxWithUser(store, testUserID)
}

func do(t *testing.T, mux *http.ServeMux, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, target, reader))
	return rec
}

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body %q: %v", rec.Body.String(), err)
	}
	return body.Error
}

func TestGetUserByIDMapsMissingUserToNotFound(t *testing.T) {
	store := &errorStore{getErr: database.ErrUserNotFound}

	req := httptest.NewRequest(http.MethodGet, "/api/user/"+testUserID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	newTestMux(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if msg := errorMessage(t, rec); msg != "User not found" {
		t.Fatalf("error = %q, want %q", msg, "User not found")
	}
}

func TestGetUserByEmailMapsMissingUserToNotFound(t *testing.T) {
	store := &errorStore{getErr: database.ErrUserNotFound}

	req := httptest.NewRequest(http.MethodGet, "/api/user?email=missing@example.com", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	newTestMux(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetUserByIDRejectsMalformedID(t *testing.T) {
	store := &errorStore{record: &database.UserRecord{ID: testUserID}}

	req := httptest.NewRequest(http.MethodGet, "/api/user/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	newTestMux(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if store.seenID != "" {
		t.Fatalf("malformed ID reached the store: %q", store.seenID)
	}
}

func TestGetUserByEmailNormalizesLookup(t *testing.T) {
	store := &errorStore{record: &database.UserRecord{ID: testUserID, Email: "ada@example.com"}}

	req := httptest.NewRequest(http.MethodGet, "/api/user?email=ADA@EXAMPLE.COM", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	newTestMux(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if store.seenEmail != "ada@example.com" {
		t.Fatalf("store lookup email = %q, want %q", store.seenEmail, "ada@example.com")
	}
}

func TestSignupDuplicateEmailIsConflict(t *testing.T) {
	store := &errorStore{createErr: database.ErrEmailConflict}

	rec := do(t, newTestMux(store), http.MethodPost, "/api/user/create",
		`{"name":"Ada","email":"ada@example.com","password":"password123"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestSignupValidationFailureIsBadRequest(t *testing.T) {
	rec := do(t, newTestMux(&errorStore{}), http.MethodPost, "/api/user/create",
		`{"name":"Ada","email":"ada@example.com","password":"short"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeleteUserMissingIsNotFound(t *testing.T) {
	store := &errorStore{deleteErr: database.ErrUserNotFound}

	req := httptest.NewRequest(http.MethodDelete, "/api/user/"+testUserID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	newTestMux(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUnexpectedStoreErrorHidesDetails(t *testing.T) {
	store := &errorStore{getErr: io.ErrUnexpectedEOF}

	req := httptest.NewRequest(http.MethodGet, "/api/user/"+testUserID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	newTestMux(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if msg := errorMessage(t, rec); msg != "Failed to retrieve user" {
		t.Fatalf("error = %q, want %q", msg, "Failed to retrieve user")
	}
}

// TestProtectedRoutesAreWrappedByMiddleware pins the wiring: the routes that
// expose or change another user's data must go through the injected middleware,
// while the public ones must not.
func TestProtectedRoutesAreWrappedByMiddleware(t *testing.T) {
	deny := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}

	store := &errorStore{record: &database.UserRecord{ID: testUserID, Email: "ada@example.com"}}
	mux := http.NewServeMux()
	NewHandler(NewService(store), deny).RegisterRoutes(mux)

	// GET /api/users is intentionally disabled (see RegisterRoutes), so it is
	// not in the protected list; re-add it here if the route returns behind a
	// role check.
	protected := []struct {
		method string
		target string
	}{
		{http.MethodPatch, "/api/user/" + testUserID},
		{http.MethodDelete, "/api/user/" + testUserID},
		{http.MethodGet, "/api/user?email=ada@example.com"},
		{http.MethodGet, "/api/user/" + testUserID},
	}

	for _, route := range protected {
		rec := do(t, mux, route.method, route.target, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", route.method, route.target, rec.Code, http.StatusUnauthorized)
		}
	}
}

func TestGetUserByIDRejectsDifferentUser(t *testing.T) {
	store := &errorStore{record: &database.UserRecord{ID: testUserID, Email: "ada@example.com"}}
	mux := http.NewServeMux()
	NewHandler(NewService(store), auth.Middleware(&staticVerifier{userID: "22222222-2222-2222-2222-222222222222"})).RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/user/"+testUserID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
