package users

import (
	"encoding/json"
	"io"
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

func newTestMux(store database.Users) *http.ServeMux {
	mux := http.NewServeMux()
	NewHandler(NewService(store)).RegisterRoutes(mux)
	return mux
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

	rec := do(t, newTestMux(store), http.MethodGet, "/api/user/"+testUserID, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if msg := errorMessage(t, rec); msg != "User not found" {
		t.Fatalf("error = %q, want %q", msg, "User not found")
	}
}

func TestGetUserByEmailMapsMissingUserToNotFound(t *testing.T) {
	store := &errorStore{getErr: database.ErrUserNotFound}

	rec := do(t, newTestMux(store), http.MethodGet, "/api/user?email=missing@example.com", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetUserByIDRejectsMalformedID(t *testing.T) {
	store := &errorStore{record: &database.UserRecord{ID: testUserID}}

	rec := do(t, newTestMux(store), http.MethodGet, "/api/user/not-a-uuid", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if store.seenID != "" {
		t.Fatalf("malformed ID reached the store: %q", store.seenID)
	}
}

func TestGetUserByEmailNormalizesLookup(t *testing.T) {
	store := &errorStore{record: &database.UserRecord{ID: testUserID, Email: "ada@example.com"}}

	rec := do(t, newTestMux(store), http.MethodGet, "/api/user?email=ADA@EXAMPLE.COM", "")

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

	rec := do(t, newTestMux(store), http.MethodDelete, "/api/user/"+testUserID, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUnexpectedStoreErrorHidesDetails(t *testing.T) {
	store := &errorStore{getErr: io.ErrUnexpectedEOF}

	rec := do(t, newTestMux(store), http.MethodGet, "/api/user/"+testUserID, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if msg := errorMessage(t, rec); msg != "Failed to retrieve user" {
		t.Fatalf("error = %q, want %q", msg, "Failed to retrieve user")
	}
}
