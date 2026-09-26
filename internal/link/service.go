package link

import (
	"errors"
	"fmt"
	"link-shortner/internal/auth"
	"link-shortner/internal/database"
	"link-shortner/internal/shortcode"
	"net/url"
	"strings"
	"time"
)

var (
	ErrMissingURL = errors.New("url is required")
	ErrInvalidURL = errors.New("url must be an absolute URL, e.g. https://example.com")
	// ErrInvalidOwner reuses the auth package's ID validation: the owner of a
	// link is always a user ID that came out of a verified token.
	ErrInvalidOwner = auth.ErrInvalidID
)

const (
	shortCodeLength = 8
	maxCodeAttempts = 5
)

type Service struct {
	baseURL  string
	database database.Link
}

type ShortenResult struct {
	ShortURL string
	Code     string
	LongURL  string
}

// LinkResult is a link as the owning client sees it: their code, their long
// URL, the public short URL, and the click count. It deliberately omits the
// owner's user ID, which the client already knows and has no need to echo.
type LinkResult struct {
	Code      string    `json:"code"`
	ShortURL  string    `json:"short_url"`
	LongURL   string    `json:"long_url"`
	Clicks    int64     `json:"clicks"`
	CreatedAt time.Time `json:"created_at"`
}

func NewService(baseURL string, storage database.Link) *Service {
	return &Service{baseURL: baseURL, database: storage}
}

// Shorten creates a short link owned by userID. The owner is stamped on the
// record at creation time — it never travels in the request body — so a link
// cannot be created on anybody else's behalf.
func (s *Service) Shorten(userID, rawURL string) (*ShortenResult, error) {
	userID, err := auth.ValidateUserID(userID)
	if err != nil {
		return nil, ErrInvalidOwner
	}
	longURL, err := normalizeURL(rawURL)
	if err != nil {
		return nil, err
	}

	var code string
	for attempt := 0; attempt < maxCodeAttempts; attempt++ {
		code, err = shortcode.RandomBase62(shortCodeLength)
		if err != nil {
			return nil, err
		}

		record := &database.URLRecord{Code: code, LongURL: longURL, UserID: userID, CreatedAt: time.Now()}
		if err = s.database.Save(record); !errors.Is(err, database.ErrCodeConflict) {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	return &ShortenResult{
		ShortURL: fmt.Sprintf("%s/%s", s.baseURL, code),
		Code:     code,
		LongURL:  longURL,
	}, nil
}

// ListByUser returns every link the user has shortened, newest first. The
// store filters by owner, so the result is only ever the caller's own links.
func (s *Service) ListByUser(userID string) ([]LinkResult, error) {
	userID, err := auth.ValidateUserID(userID)
	if err != nil {
		return nil, ErrInvalidOwner
	}

	recs, err := s.database.GetByUser(userID)
	if err != nil {
		return nil, err
	}

	results := make([]LinkResult, 0, len(recs))
	for _, rec := range recs {
		results = append(results, toLinkResult(s.baseURL, rec))
	}
	return results, nil
}

// StatsForUser returns a link's stats, but only to its owner. A link that
// belongs to somebody else answers ErrNotFound, the same as a code that was
// never issued: the response teaches the caller nothing about codes they do
// not own.
func (s *Service) StatsForUser(userID, code string) (*LinkResult, error) {
	userID, err := auth.ValidateUserID(userID)
	if err != nil {
		return nil, ErrInvalidOwner
	}

	rec, err := s.database.Get(code)
	if err != nil {
		return nil, err
	}
	if rec.UserID != userID {
		return nil, database.ErrNotFound
	}

	result := toLinkResult(s.baseURL, *rec)
	return &result, nil
}

func (s *Service) Resolve(code string) (*database.URLRecord, error) {
	// The store records the click atomically, so a failing analytics write is
	// surfaced instead of being silently dropped.
	// Resolve stays public and owner-blind on purpose: the whole point of a
	// short link is that anybody who gets the code can follow it.
	return s.database.GetAndIncrement(code)
}

func toLinkResult(baseURL string, rec database.URLRecord) LinkResult {
	return LinkResult{
		Code:      rec.Code,
		ShortURL:  fmt.Sprintf("%s/%s", baseURL, rec.Code),
		LongURL:   rec.LongURL,
		Clicks:    rec.Clicks,
		CreatedAt: rec.CreatedAt,
	}
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrMissingURL
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", ErrInvalidURL
	}
	return raw, nil
}
