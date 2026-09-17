package link

import (
	"errors"
	"link-shortner/internal/database"
	"testing"
)

type fakeLinkStore struct {
	saveErrs     []error
	saveCalls    int
	saved        []*database.URLRecord
	getErr       error
	getCalls     int
	resolveErr   error
	resolveCalls int
	resolvedCode string
	resolved     *database.URLRecord
}

func (s *fakeLinkStore) Save(rec *database.URLRecord) error {
	s.saveCalls++

	var err error
	if len(s.saveErrs) > 0 {
		err = s.saveErrs[0]
		s.saveErrs = s.saveErrs[1:]
	}
	if err != nil {
		return err
	}

	s.saved = append(s.saved, rec)
	return nil
}

func (s *fakeLinkStore) Get(string) (*database.URLRecord, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	if len(s.saved) == 0 {
		return nil, database.ErrNotFound
	}
	return s.saved[len(s.saved)-1], nil
}

func (s *fakeLinkStore) GetAndIncrement(code string) (*database.URLRecord, error) {
	s.resolveCalls++
	s.resolvedCode = code
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}

	// Like the real store, resolve the code from what was saved and return the
	// row with the click already counted.
	for _, rec := range s.saved {
		if rec.Code == code {
			rec.Clicks++
			s.resolved = rec
			return rec, nil
		}
	}
	return nil, database.ErrNotFound
}

func TestShortenRetriesUntilCodeIsFree(t *testing.T) {
	store := &fakeLinkStore{saveErrs: []error{database.ErrCodeConflict, database.ErrCodeConflict, nil}}
	service := NewService("http://localhost:8080", store)

	result, err := service.Shorten("https://example.com/path")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if store.saveCalls != 3 {
		t.Fatalf("save attempts = %d, want 3", store.saveCalls)
	}
	if len(result.Code) != shortCodeLength {
		t.Fatalf("code = %q, want length %d", result.Code, shortCodeLength)
	}
	if result.LongURL != "https://example.com/path" {
		t.Fatalf("long URL = %q", result.LongURL)
	}
	if want := "http://localhost:8080/" + result.Code; result.ShortURL != want {
		t.Fatalf("short URL = %q, want %q", result.ShortURL, want)
	}
}

func TestShortenGivesUpAfterMaxAttempts(t *testing.T) {
	store := &fakeLinkStore{saveErrs: []error{
		database.ErrCodeConflict,
		database.ErrCodeConflict,
		database.ErrCodeConflict,
		database.ErrCodeConflict,
		database.ErrCodeConflict,
	}}
	service := NewService("http://localhost:8080", store)

	_, err := service.Shorten("https://example.com")
	if !errors.Is(err, database.ErrCodeConflict) {
		t.Fatalf("Shorten() error = %v, want %v", err, database.ErrCodeConflict)
	}
	if store.saveCalls != maxCodeAttempts {
		t.Fatalf("save attempts = %d, want %d", store.saveCalls, maxCodeAttempts)
	}
}

func TestShortenValidation(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{name: "empty", rawURL: "", wantErr: ErrMissingURL},
		{name: "whitespace", rawURL: "   ", wantErr: ErrMissingURL},
		{name: "no scheme", rawURL: "example.com", wantErr: ErrInvalidURL},
		{name: "unsupported scheme", rawURL: "ftp://example.com", wantErr: ErrInvalidURL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeLinkStore{}
			service := NewService("http://localhost:8080", store)

			_, err := service.Shorten(test.rawURL)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Shorten() error = %v, want %v", err, test.wantErr)
			}
			if store.saveCalls != 0 {
				t.Fatalf("save attempts = %d, want 0", store.saveCalls)
			}
		})
	}
}

func TestShortenTrimsWhitespace(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)

	result, err := service.Shorten("  https://example.com/trimmed  ")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if result.LongURL != "https://example.com/trimmed" {
		t.Fatalf("long URL = %q, want trimmed value", result.LongURL)
	}
}

func TestResolveUsesAtomicIncrement(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)

	result, err := service.Shorten("https://example.com/target")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}

	first, err := service.Resolve(result.Code)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	firstClicks := first.Clicks

	second, err := service.Resolve(result.Code)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if store.getCalls != 0 {
		t.Fatalf("Get() calls = %d, want 0 (resolve must not read then increment)", store.getCalls)
	}
	if store.resolveCalls != 2 {
		t.Fatalf("GetAndIncrement() calls = %d, want 2", store.resolveCalls)
	}
	if store.resolvedCode != result.Code {
		t.Fatalf("resolved code = %q, want %q", store.resolvedCode, result.Code)
	}
	if firstClicks != 1 {
		t.Fatalf("clicks after first resolve = %d, want 1", firstClicks)
	}
	if second.Clicks != firstClicks+1 {
		t.Fatalf("clicks = %d then %d, want an increment per resolve", firstClicks, second.Clicks)
	}
	if second.LongURL != "https://example.com/target" {
		t.Fatalf("long URL = %q", second.LongURL)
	}
}

func TestResolvePropagatesStoreErrors(t *testing.T) {
	store := &fakeLinkStore{resolveErr: database.ErrNotFound}
	service := NewService("http://localhost:8080", store)

	_, err := service.Resolve("missing1")
	if !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want %v", err, database.ErrNotFound)
	}
}

func TestStatsDoesNotCountClicks(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)

	if _, err := service.Shorten("https://example.com"); err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	rec, err := service.Stats("whatever")
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if store.resolveCalls != 0 {
		t.Fatalf("GetAndIncrement() calls = %d, want 0 for stats", store.resolveCalls)
	}
	if rec.Clicks != 0 {
		t.Fatalf("clicks = %d, want 0", rec.Clicks)
	}
}
