package link

import (
	"encoding/json"
	"errors"
	"io"
	"link-shortner/internal/auth"
	"link-shortner/internal/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// staticVerifier stands in for the real token verifier: it accepts any bearer
// token and reports a fixed user, so tests choose who is calling without
// signing JWTs.
type staticVerifier struct {
	userID string
}

func (v *staticVerifier) Verify(string) (*auth.Claims, error) {
	return &auth.Claims{UserID: v.userID}, nil
}

// denyAll stands in for the real middleware when a test must prove a route is
// wrapped: it rejects every request before the handler runs.
func denyAll(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// newTestMux wires the routes with testUserID as the caller.
func newTestMux(store database.Link) *http.ServeMux {
	return newTestMuxWithUser(store, testUserID)
}

// newTestMuxWithUser wires the routes with an arbitrary caller.
func newTestMuxWithUser(store database.Link, userID string) *http.ServeMux {
	mux := http.NewServeMux()
	NewHandler(NewService("http://localhost:8080", store), auth.Middleware(&staticVerifier{userID: userID})).RegisterRoutes(mux)
	return mux
}

func do(t *testing.T, mux *http.ServeMux, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, reader)
	// Every request carries a bearer token; the injected verifier decides
	// which user it belongs to, and the public routes ignore it.
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestShortenRouteCreatesLink(t *testing.T) {
	store := &fakeLinkStore{}

	rec := do(t, newTestMux(store), http.MethodPost, "/api/shorten", `{"url":"https://example.com"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// The saved record must carry the caller as its owner.
	if len(store.saved) != 1 || store.saved[0].UserID != testUserID {
		t.Fatalf("saved owner = %+v, want %q", store.saved, testUserID)
	}

	var body struct {
		ShortURL string `json:"short_url"`
		Code     string `json:"code"`
		LongURL  string `json:"long_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body %q: %v", rec.Body.String(), err)
	}
	if body.LongURL != "https://example.com" {
		t.Fatalf("long_url = %q", body.LongURL)
	}
	if body.Code == "" || body.ShortURL != "http://localhost:8080/"+body.Code {
		t.Fatalf("code/short_url mismatch: %#v", body)
	}
}

// TestProtectedLinkRoutesAreWrappedByMiddleware pins the wiring: shorten,
// listing, and stats must all go through the injected middleware, or an
// unauthenticated caller could create links and read other users' data.
func TestProtectedLinkRoutesAreWrappedByMiddleware(t *testing.T) {
	store := &fakeLinkStore{}
	mux := http.NewServeMux()
	NewHandler(NewService("http://localhost:8080", store), denyAll).RegisterRoutes(mux)

	protected := []struct {
		method string
		target string
		body   string
	}{
		{http.MethodPost, "/api/shorten", `{"url":"https://example.com"}`},
		{http.MethodGet, "/api/links", ""},
		{http.MethodGet, "/api/stats/abcd1234", ""},
	}

	for _, route := range protected {
		rec := do(t, mux, route.method, route.target, route.body)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", route.method, route.target, rec.Code, http.StatusUnauthorized)
		}
	}
	if store.saveCalls != 0 {
		t.Fatalf("save attempts = %d, want 0", store.saveCalls)
	}
}

func TestShortenRouteRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"url":`},
		{name: "relative url", body: `{"url":"example.com"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeLinkStore{}

			rec := do(t, newTestMux(store), http.MethodPost, "/api/shorten", test.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if store.saveCalls != 0 {
				t.Fatalf("save attempts = %d, want 0", store.saveCalls)
			}
		})
	}
}

func TestShortenRouteHidesStoreFailure(t *testing.T) {
	store := &fakeLinkStore{saveErrs: []error{errors.New("connection reset")}}

	rec := do(t, newTestMux(store), http.MethodPost, "/api/shorten", `{"url":"https://example.com"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Fatalf("response leaked the store error: %q", rec.Body.String())
	}
}

func TestRedirectRecordsClick(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)
	if _, err := service.Shorten(testUserID, "https://example.com/target"); err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(service, auth.Middleware(&staticVerifier{userID: testUserID})).RegisterRoutes(mux)

	rec := do(t, mux, http.MethodGet, "/"+store.saved[0].Code, "")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if location := rec.Header().Get("Location"); location != "https://example.com/target" {
		t.Fatalf("Location = %q", location)
	}
	if store.resolveCalls != 1 {
		t.Fatalf("GetAndIncrement() calls = %d, want 1", store.resolveCalls)
	}
	if store.resolved.Clicks != 1 {
		t.Fatalf("clicks = %d, want 1", store.resolved.Clicks)
	}
}

func TestRedirectUnknownCodeIsNotFound(t *testing.T) {
	store := &fakeLinkStore{resolveErr: database.ErrNotFound}

	rec := do(t, newTestMux(store), http.MethodGet, "/missing1", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "short link not found") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestStatsRouteDoesNotCountClicks(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)
	result, err := service.Shorten(testUserID, "https://example.com/target")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(service, auth.Middleware(&staticVerifier{userID: testUserID})).RegisterRoutes(mux)

	rec := do(t, mux, http.MethodGet, "/api/stats/"+result.Code, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if store.resolveCalls != 0 {
		t.Fatalf("GetAndIncrement() calls = %d, want 0 for stats", store.resolveCalls)
	}
}
