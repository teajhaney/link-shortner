package store

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("short code not found")
	ErrCodeConflict = errors.New("short code already exists")
)

// records for everything tracked by the system for a given shortlinked
type URLRecord struct {
	Code      string
	LongURL   string
	CreatedAt time.Time
	Clicks    int64
}

// Store is the storage contract. Anything satisfying this interface
// (in-memory, Redis, Postgres, ...) can back the shortener without
// the HTTP handlers needing to change.
type Store interface {
	Save(rec *URLRecord) error
	Get(code string) (*URLRecord, error)
	IncrementClicks(code string) error
	NextID() uint64
}

// Memory is a concurrency-safe, in-memory Store.
// Data is lost on restart -- fine for learning, swap out for
// a real database when you want persistence.
type Memory struct {
	mu      sync.RWMutex
	records map[string]*URLRecord
	counter uint64
}
