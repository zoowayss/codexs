package codexs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type SessionCache struct {
	Version  int               `json:"version"`
	Sessions map[string]string `json:"sessions"` // sessionID -> profileName
}

func newSessionCache() SessionCache {
	return SessionCache{
		Version:  1,
		Sessions: make(map[string]string),
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
		cache.Sessions = make(map[string]string)
	}
	return cache, nil
}

func (s *Store) SaveSessionCache(cache SessionCache) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", s.root, err)
	}
	if cache.Sessions == nil {
		cache.Sessions = make(map[string]string)
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session cache: %w", err)
	}
	data = append(data, '\n')
	path := s.SessionCachePath()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (s *Store) AddSessionToCache(sessionID, profileName string) error {
	cache, err := s.LoadSessionCache()
	if err != nil {
		return err
	}
	cache.Sessions[sessionID] = profileName
	return s.SaveSessionCache(cache)
}
