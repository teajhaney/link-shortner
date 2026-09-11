package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"link-shortner/internal/shortcode"
	"link-shortner/internal/store"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Server struct {
	store   store.Store
	baseURL string
}

func NewServer(baseURL string, store store.Store) *Server {
	return &Server{
		store:   store,
		baseURL: baseURL,
	}
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shorten", s.handleShorten)
	mux.HandleFunc("GET /api/stats/{code}", s.handleStats)
	mux.HandleFunc("GET /{code}", s.handleRedirect)
	return mux
}

// write json response
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// write error response
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// normalize url
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("url is required")
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("url must be an absolute URL, e.g. https://example.com")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}

	return raw, nil
}

// shorten request
type shortenRequest struct {
	URL string `json:"url"`
}

// shorten response
type shortenResponse struct {
	ShortURL string `json:"short_url"`
	Code     string `json:"code"`
	LongURL  string `json:"long_url"`
}

const shortCodeLength = 8
const maxCodeAttempts = 5

// handle shorten request
func (s *Server) handleShorten(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	longURL, err := normalizeURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var code string
	var rec *store.URLRecord
	for attempt := 0; attempt < maxCodeAttempts; attempt++ {
		code, err = shortcode.RandomBase62(shortCodeLength)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to generate short code")
			return
		}

		rec = &store.URLRecord{
			Code:      code,
			LongURL:   longURL,
			CreatedAt: time.Now(),
		}

		err = s.store.Save(rec)
		if !errors.Is(err, store.ErrCodeConflict) {
			break
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save URL record")
		return
	}

	shortURL := fmt.Sprintf("%s/%s", s.baseURL, code)
	writeJSON(w, http.StatusCreated, shortenResponse{ShortURL: shortURL, Code: code, LongURL: longURL})
}

// handle redirect request
func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	rec, err := s.store.Get(code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "short link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	_ = s.store.IncrementClicks(code)

	http.Redirect(w, r, rec.LongURL, http.StatusFound)
}

// handle stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	rec, err := s.store.Get(code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "short link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	writeJSON(w, http.StatusOK, rec)
}
