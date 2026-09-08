package store

import (
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("Short code not found")


//record sfor everything tracked by the system for a given shortlinked
type URLRecord struct {
	Code string
	LongURL string
	CreatedAt time.Time
	Clicks int64
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
	mu sync.RWMutex
	records map[string]*URLRecord
	counter uint64
}

// func NewMemory() *Memory {
// 	return &Memory{
// 		records: make(map[string]*URLRecord),
// 	}
// }


// func (s *Memory) Save(rec *URLRecord) error {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	if _, exists := s.records[rec.Code]; exists {
// 		return errors.New("short code already exists")
// 	}
// 	rec.CreatedAt = time.Now()
// 	rec.Clicks = 0
// 	s.records[rec.Code] = rec
// 	return nil
// }


// func (s *Memory) Get(code string) (*URLRecord, error) {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()
// 	rec, ok := s.records[code]
// 	if !ok {
// 		return nil, ErrNotFound
// 	}
// 	return rec, nil
// }

// func (s *Memory) IncrementClicks(code string) error {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	rec, ok := s.records[code]
// 	if !ok {
// 		return ErrNotFound
// 	}
// 	rec.Clicks++
// 	return nil
// }

// func (s *Memory) NextID() uint64 {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	s.counter++
// 	return s.counter
// }

