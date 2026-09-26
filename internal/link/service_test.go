package link

import (
	"errors"
	"link-shortner/internal/database"
	"testing"
)

// testUserID and otherUserID are two distinct users used to pin the ownership
// behavior: links created by one must never be visible to the other.
const (
	testUserID  = "11111111-1111-1111-1111-111111111111"
	otherUserID = "22222222-2222-2222-2222-222222222222"
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
	listCalls    int
	listUserID   string
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

// GetByUser mirrors the real store: it filters by owner, so ownerless rows
// and other users' rows are both invisible.
func (s *fakeLinkStore) GetByUser(userID string) ([]database.URLRecord, error) {
	s.listCalls++
	s.listUserID = userID

	records := make([]database.URLRecord, 0)
	for _, rec := range s.saved {
		if rec.UserID == userID {
			records = append(records, *rec)
		}
	}
	return records, nil
}

func TestShortenRetriesUntilCodeIsFree(t *testing.T) {
	store := &fakeLinkStore{saveErrs: []error{database.ErrCodeConflict, database.ErrCodeConflict, nil}}
	service := NewService("http://localhost:8080", store)

	result, err := service.Shorten(testUserID, "https://example.com/path")
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

	_, err := service.Shorten(testUserID, "https://example.com")
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

			_, err := service.Shorten(testUserID, test.rawURL)
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

	result, err := service.Shorten(testUserID, "  https://example.com/trimmed  ")
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

	result, err := service.Shorten(testUserID, "https://example.com/target")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if store.saved[0].UserID != testUserID {
		t.Fatalf("saved owner = %q, want %q", store.saved[0].UserID, testUserID)
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

func TestStatsForUserReturnsOwnersLink(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)

	result, err := service.Shorten(testUserID, "https://example.com")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}

	stats, err := service.StatsForUser(testUserID, result.Code)
	if err != nil {
		t.Fatalf("StatsForUser() error = %v", err)
	}
	// Stats reads without counting: the owner can check their numbers
	// without inflating them.
	if store.resolveCalls != 0 {
		t.Fatalf("GetAndIncrement() calls = %d, want 0 for stats", store.resolveCalls)
	}
	if stats.Clicks != 0 {
		t.Fatalf("clicks = %d, want 0", stats.Clicks)
	}
	if stats.LongURL != "https://example.com" {
		t.Fatalf("long URL = %q", stats.LongURL)
	}
	if stats.ShortURL != "http://localhost:8080/"+result.Code {
		t.Fatalf("short URL = %q", stats.ShortURL)
	}
}

func TestStatsForUserHidesOtherUsersLinks(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)

	result, err := service.Shorten(testUserID, "https://example.com/private")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}

	// The same lookup as another user must answer not-found: the code
	// exists, but the caller has no way to learn that.
	_, err = service.StatsForUser(otherUserID, result.Code)
	if !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("StatsForUser() error = %v, want %v", err, database.ErrNotFound)
	}
}

func TestListByUserReturnsOnlyOwnersLinks(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)

	for _, raw := range []string{"https://example.com/a", "https://example.com/b"} {
		if _, err := service.Shorten(testUserID, raw); err != nil {
			t.Fatalf("Shorten(%q) error = %v", raw, err)
		}
	}
	if _, err := service.Shorten(otherUserID, "https://example.com/theirs"); err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}

	mine, err := service.ListByUser(testUserID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("links = %d, want 2 (the other user's link must be invisible)", len(mine))
	}
	for _, link := range mine {
		if link.LongURL == "https://example.com/theirs" {
			t.Fatalf("leaked another user's link: %+v", link)
		}
	}

	theirs, err := service.ListByUser(otherUserID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(theirs) != 1 || theirs[0].LongURL != "https://example.com/theirs" {
		t.Fatalf("other user's links = %+v, want exactly their own link", theirs)
	}
}

func TestShortenRejectsInvalidOwner(t *testing.T) {
	store := &fakeLinkStore{}
	service := NewService("http://localhost:8080", store)

	// The middleware normally guarantees a well-formed user ID; this pins
	// that a malformed one is rejected before anything is stored.
	_, err := service.Shorten("not-a-uuid", "https://example.com")
	if !errors.Is(err, ErrInvalidOwner) {
		t.Fatalf("Shorten() error = %v, want %v", err, ErrInvalidOwner)
	}
	if store.saveCalls != 0 {
		t.Fatalf("save attempts = %d, want 0", store.saveCalls)
	}
}
