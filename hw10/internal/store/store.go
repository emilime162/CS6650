// Package store provides a thread-safe, in-memory key-value store
// where every entry carries a logical version number.
//
// Version numbers are per-key and increment on every write. They are
// used by the leader and coordinators to decide which copy of a value
// is "most recent" when collecting results from multiple nodes (R > 1).
package store

import "sync"

// Entry holds a stored value together with its logical version.
// Version starts at 1 on the first write and increases by 1 on every
// subsequent write to the same key.
type Entry struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// Store is a goroutine-safe in-memory key-value map.
type Store struct {
	mu   sync.RWMutex
	data map[string]*Entry
}

// New returns an empty Store ready for use.
func New() *Store {
	return &Store{data: make(map[string]*Entry)}
}

// Set writes value under key and returns the new version number.
// If the key does not exist yet, version 1 is assigned.
// If an incoming version is supplied (> 0) and is higher than the
// local version, the incoming version wins (used during replication).
func (s *Store) Set(key, value string, incomingVersion int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.data[key]
	var newVersion int64
	if !ok {
		// First write for this key.
		if incomingVersion > 0 {
			newVersion = incomingVersion
		} else {
			newVersion = 1
		}
	} else {
		// Subsequent write: use the larger of (local+1) or incomingVersion.
		local := existing.Version + 1
		if incomingVersion > local {
			newVersion = incomingVersion
		} else {
			newVersion = local
		}
	}

	s.data[key] = &Entry{Value: value, Version: newVersion}
	return newVersion
}

// Get returns the Entry for key, or (nil, false) if the key is absent.
func (s *Store) Get(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok {
		return nil, false
	}
	// Return a copy so the caller cannot mutate store internals.
	copy := *e
	return &copy, true
}
