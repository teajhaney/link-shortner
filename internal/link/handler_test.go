package link

import (
	"encoding/json"
	"errors"
	"io"
	"link-shortner/internal/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestMux(store database.Link) *http.ServeMux {
	mux := http.NewServeMux()
	NewHandler(NewService("http://localhost:8080", store)).RegisterRoutes(mux)
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

func TestShortenRouteCreatesLink(t *testing.T) {
	store := &fakeLinkStore{}

	rec := do(t, newTestMux(store), http.MethodPost, "/api/shorten", `{"url":"https://example.com"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
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
	if _, err := service.Shorten("https://example.com/target"); err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

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
	result, err := service.Shorten("https://example.com/target")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	rec := do(t, mux, http.MethodGet, "/api/stats/"+result.Code, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if store.resolveCalls != 0 {
		t.Fatalf("GetAndIncrement() calls = %d, want 0 for stats", store.resolveCalls)
	}
}
