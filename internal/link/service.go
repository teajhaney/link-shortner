package link

import (
	"errors"
	"fmt"
	"link-shortner/internal/database"
	"link-shortner/internal/shortcode"
	"net/url"
	"strings"
	"time"
)

var (
	ErrMissingURL = errors.New("url is required")
	ErrInvalidURL = errors.New("url must be an absolute URL, e.g. https://example.com")
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

func NewService(baseURL string, storage database.Link) *Service {
	return &Service{baseURL: baseURL, database: storage}
}

func (s *Service) Shorten(rawURL string) (*ShortenResult, error) {
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

		record := &database.URLRecord{Code: code, LongURL: longURL, CreatedAt: time.Now()}
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

func (s *Service) Resolve(code string) (*database.URLRecord, error) {
	// The store records the click atomically, so a failing analytics write is
	// surfaced instead of being silently dropped.
	return s.database.GetAndIncrement(code)
}

func (s *Service) Stats(code string) (*database.URLRecord, error) {
	return s.database.Get(code)
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
