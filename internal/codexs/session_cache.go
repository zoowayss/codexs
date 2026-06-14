package codexs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// sessionAccessRefresh is the minimum age of a cached entry's LastAccess before
// a cache hit bothers to rewrite the cache. It keeps resume on the read path
// cheap while still refreshing entries often enough relative to the prune TTL.
const sessionAccessRefresh = 24 * time.Hour

type SessionCacheEntry struct {
	ProfileName string    `json:"profile_name"`
	LastAccess  time.Time `json:"last_access"`
}

type SessionCache struct {
	Version  int                          `json:"version"`
	Sessions map[string]SessionCacheEntry `json:"sessions"` // sessionID -> entry
}

func newSessionCache() SessionCache {
	return SessionCache{
		Version:  1,
		Sessions: make(map[string]SessionCacheEntry),
	}
}

func (s *Store) SessionCachePath() string {
	return filepath.Join(s.root, "sessions_cache.json")
}

func (s *Store) LoadSessionCache() (SessionCache, error) {
	path := s.SessionCachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newSessionCache(), nil
		}
		return SessionCache{}, fmt.Errorf("read %s: %w", path, err)
	}
	var cache SessionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return SessionCache{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if cache.Sessions == nil {
		cache.Sessions = make(map[string]SessionCacheEntry)
	}
	return cache, nil
}

func (s *Store) SaveSessionCache(cache SessionCache) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", s.root, err)
	}
	if cache.Sessions == nil {
		cache.Sessions = make(map[string]SessionCacheEntry)
	}
	pruneExpiredSessions(cache)
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session cache: %w", err)
	}
	data = append(data, '\n')
	path := s.SessionCachePath()
	if err := writeFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// pruneExpiredSessions drops entries not accessed within the retention window.
// Pruning happens at save time so reads stay side-effect free and every write
// keeps the cache bounded.
func pruneExpiredSessions(cache SessionCache) {
	const maxAge = 30 * 24 * time.Hour
	cutoff := time.Now().Add(-maxAge)
	for sessionID, entry := range cache.Sessions {
		if entry.LastAccess.Before(cutoff) {
			delete(cache.Sessions, sessionID)
		}
	}
}
