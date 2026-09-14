package link

import (
	"errors"
	"fmt"
	"link-shortner/internal/shortcode"
	"link-shortner/internal/store"
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
	baseURL string
	store   store.Store
}

type ShortenResult struct {
	ShortURL string
	Code     string
	LongURL  string
}

func NewService(baseURL string, storage store.Store) *Service {
	return &Service{baseURL: baseURL, store: storage}
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

		record := &store.URLRecord{Code: code, LongURL: longURL, CreatedAt: time.Now()}
		if err = s.store.Save(record); !errors.Is(err, store.ErrCodeConflict) {
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

func (s *Service) Resolve(code string) (*store.URLRecord, error) {
	record, err := s.store.Get(code)
	if err != nil {
		return nil, err
	}
	_ = s.store.IncrementClicks(code)
	return record, nil
}

func (s *Service) Stats(code string) (*store.URLRecord, error) {
	return s.store.Get(code)
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
