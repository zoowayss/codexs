package codexs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	root string
}

func NewStoreForPlatform(platform Platform) (*Store, error) {
	root, err := defaultStoreRoot(platform)
	if err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func NewStoreAt(root string) *Store {
	return &Store{root: root}
}

func defaultStoreRoot(platform Platform) (string, error) {
	if root := os.Getenv("CODEXS_HOME"); root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("resolve CODEXS_HOME: %w", err)
		}
		return abs, nil
	}
	return platform.DefaultStoreRoot(appID)
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) StatePath() string {
	return filepath.Join(s.root, "profiles.json")
}

func (s *Store) ProfileRoot(name string) string {
	return filepath.Join(s.root, "profiles", name)
}

func (s *Store) CodexHome(name string) string {
	return filepath.Join(s.ProfileRoot(name), "codex-home")
}

func (s *Store) Load() (State, error) {
	path := s.StatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newState(), nil
		}
		return State{}, fmt.Errorf("read %s: %w", path, err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if state.Version == 0 {
		state.Version = stateVersion
	}
	if state.Profiles == nil {
		state.Profiles = make(map[string]Profile)
	}
	return state, nil
}

func (s *Store) Save(state State) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", s.root, err)
	}
	state.Version = stateVersion
	if state.Profiles == nil {
		state.Profiles = make(map[string]Profile)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profiles: %w", err)
	}
	data = append(data, '\n')
	path := s.StatePath()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (s *Store) Ensure() error {
	state, err := s.Load()
	if err != nil {
		return err
	}
	return s.Save(state)
}

func (s *Store) AddProfile(name, baseURL, apiKey string, now time.Time) (Profile, error) {
	if err := validateProfileName(name); err != nil {
		return Profile{}, err
	}
	normalizedBaseURL, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Profile{}, err
	}
	if apiKey == "" {
		return Profile{}, errors.New("api key is required")
	}
	state, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	if _, exists := state.Profiles[name]; exists {
		return Profile{}, ErrProfileExists{Name: name}
	}
	profile := Profile{
		Name:      name,
		BaseURL:   normalizedBaseURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.writeProfileFiles(profile, apiKey, true); err != nil {
		return Profile{}, err
	}
	state.Profiles[name] = profile
	if err := s.Save(state); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s *Store) AddProfileFromTemplate(name, baseURL, apiKey, sourceProfileName string, now time.Time) (Profile, error) {
	if err := validateProfileName(name); err != nil {
		return Profile{}, err
	}
	state, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	if _, exists := state.Profiles[name]; exists {
		return Profile{}, ErrProfileExists{Name: name}
	}
	sourceProfile, exists := state.Profiles[sourceProfileName]
	if !exists {
		return Profile{}, ErrProfileNotFound{Name: sourceProfileName}
	}

	finalBaseURL := baseURL
	if finalBaseURL == "" {
		finalBaseURL = sourceProfile.BaseURL
	} else {
		normalizedBaseURL, err := normalizeBaseURL(baseURL)
		if err != nil {
			return Profile{}, err
		}
		finalBaseURL = normalizedBaseURL
	}

	profile := Profile{
		Name:      name,
		BaseURL:   finalBaseURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	sourceHome := s.CodexHome(sourceProfile.Name)
	targetHome := s.CodexHome(profile.Name)
	if err := os.MkdirAll(targetHome, 0o700); err != nil {
		return Profile{}, fmt.Errorf("create %s: %w", targetHome, err)
	}
	if err := s.copyProfileFiles(sourceHome, targetHome, finalBaseURL); err != nil {
		return Profile{}, err
	}

	if apiKey != "" {
		if err := writeAuthFile(targetHome, apiKey); err != nil {
			return Profile{}, err
		}
	} else {
		sourceAuthPath := filepath.Join(sourceHome, "auth.json")
		if data, err := os.ReadFile(sourceAuthPath); err == nil {
			targetAuthPath := filepath.Join(targetHome, "auth.json")
			if err := os.WriteFile(targetAuthPath, data, 0o600); err != nil {
				return Profile{}, fmt.Errorf("write %s: %w", targetAuthPath, err)
			}
		}
	}

	state.Profiles[name] = profile
	if err := s.Save(state); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s *Store) UpdateProfile(name string, baseURL string, apiKey *string, now time.Time) (Profile, error) {
	if err := validateProfileName(name); err != nil {
		return Profile{}, err
	}
	state, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	profile, exists := state.Profiles[name]
	if !exists {
		return Profile{}, ErrProfileNotFound{Name: name}
	}

	// Update base URL if provided
	if baseURL != "" {
		normalizedBaseURL, err := normalizeBaseURL(baseURL)
		if err != nil {
			return Profile{}, err
		}
		profile.BaseURL = normalizedBaseURL
	}

	// Update API key if provided
	var key string
	if apiKey != nil {
		if *apiKey == "" {
			return Profile{}, errors.New("api key is required when updating api key")
		}
		key = *apiKey
	}

	// Write profile files (config and/or auth)
	if err := s.writeProfileFiles(profile, key, baseURL != ""); err != nil {
		return Profile{}, err
	}

	profile.UpdatedAt = now
	state.Profiles[name] = profile
	if err := s.Save(state); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s *Store) DeleteProfile(name string, purge bool) error {
	if err := validateProfileName(name); err != nil {
		return err
	}
	state, err := s.Load()
	if err != nil {
		return err
	}
	if _, exists := state.Profiles[name]; !exists {
		return ErrProfileNotFound{Name: name}
	}
	delete(state.Profiles, name)
	if err := s.Save(state); err != nil {
		return err
	}
	if purge {
		if err := os.RemoveAll(s.ProfileRoot(name)); err != nil {
			return fmt.Errorf("remove %s: %w", s.ProfileRoot(name), err)
		}
	}
	return nil
}

func (s *Store) GetProfile(name string) (Profile, error) {
	if err := validateProfileName(name); err != nil {
		return Profile{}, err
	}
	state, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	profile, exists := state.Profiles[name]
	if !exists {
		return Profile{}, ErrProfileNotFound{Name: name}
	}
	return profile, nil
}

func (s *Store) ListProfiles() ([]Profile, error) {
	state, err := s.Load()
	if err != nil {
		return nil, err
	}
	profiles := make([]Profile, 0, len(state.Profiles))
	for _, profile := range state.Profiles {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

func (s *Store) PrepareProfile(profile Profile) error {
	return s.writeProfileFiles(profile, "", false)
}

func (s *Store) writeProfileFiles(profile Profile, apiKey string, updateConfig bool) error {
	codexHome := s.CodexHome(profile.Name)
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", codexHome, err)
	}

	// Write config.toml when explicitly updating, or when it doesn't exist yet.
	configPath := filepath.Join(codexHome, "config.toml")
	if _, err := os.Stat(configPath); updateConfig || os.IsNotExist(err) {
		if err := writeConfigFile(codexHome, profile.BaseURL); err != nil {
			return err
		}
	}

	if apiKey != "" {
		if err := writeAuthFile(codexHome, apiKey); err != nil {
			return err
		}
	}
	return nil
}

// writeConfigFile renders and writes config.toml into a codex home directory.
func writeConfigFile(codexHome, baseURL string) error {
	configPath := filepath.Join(codexHome, "config.toml")
	if err := os.WriteFile(configPath, []byte(renderCodexConfig(baseURL)), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

// writeAuthFile writes auth.json containing the OpenAI API key.
func writeAuthFile(codexHome, apiKey string) error {
	auth := map[string]string{"OPENAI_API_KEY": apiKey}
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return fmt.Errorf("encode auth: %w", err)
	}
	data = append(data, '\n')
	authPath := filepath.Join(codexHome, "auth.json")
	if err := os.WriteFile(authPath, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", authPath, err)
	}
	return nil
}

func (s *Store) FindProfileForSession(sessionID string) (string, error) {
	if sessionID == "" {
		return "", errors.New("session ID is required")
	}

	// Load the cache once; reuse it for both the lookup and any update below.
	cache, err := s.LoadSessionCache()
	if err != nil {
		cache = newSessionCache()
	}

	// Try cache first.
	if entry, ok := cache.Sessions[sessionID]; ok {
		// Verify the profile still exists before trusting the cached mapping.
		if _, err := s.GetProfile(entry.ProfileName); err == nil {
			entry.LastAccess = time.Now()
			cache.Sessions[sessionID] = entry
			_ = s.SaveSessionCache(cache)
			return entry.ProfileName, nil
		}
		// Profile no longer exists; drop the stale entry and rescan.
		delete(cache.Sessions, sessionID)
	}

	// Cache miss or invalid, scan all profiles.
	state, err := s.Load()
	if err != nil {
		return "", err
	}

	for profileName := range state.Profiles {
		sessionsDir := filepath.Join(s.CodexHome(profileName), "sessions")
		if _, err := os.Stat(sessionsDir); err != nil {
			continue // Skip if sessions directory doesn't exist
		}
		// Walk through sessions directory to find files containing this sessionID
		found, err := s.sessionExistsInProfile(sessionsDir, sessionID)
		if err != nil {
			continue // Skip on error, try next profile
		}
		if found {
			// Record the mapping for next time.
			cache.Sessions[sessionID] = SessionCacheEntry{
				ProfileName: profileName,
				LastAccess:  time.Now(),
			}
			_ = s.SaveSessionCache(cache)
			return profileName, nil
		}
	}
	return "", ErrSessionNotFound{SessionID: sessionID}
}

func (s *Store) sessionExistsInProfile(sessionsDir, sessionID string) (bool, error) {
	// Session files are stored as: sessions/YYYY/MM/DD/rollout-YYYY-MM-DDTHH-MM-SS-<sessionID>.jsonl
	// Use WalkDir instead of Walk for better performance (2-3x faster)
	var found bool
	err := filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking on error
		}
		if d.IsDir() {
			return nil
		}
		// Check if filename contains the sessionID
		if strings.Contains(d.Name(), sessionID) && strings.HasSuffix(d.Name(), ".jsonl") {
			found = true
			return filepath.SkipAll // Stop walking once found
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

func (s *Store) copyProfileFiles(sourceHome, targetHome, newBaseURL string) error {
	agentsPath := filepath.Join(sourceHome, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil {
		targetPath := filepath.Join(targetHome, "AGENTS.md")
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
	}
	skillsPath := filepath.Join(sourceHome, "skills")
	if info, err := os.Stat(skillsPath); err == nil && info.IsDir() {
		targetSkillsPath := filepath.Join(targetHome, "skills")
		if err := copyDir(skillsPath, targetSkillsPath); err != nil {
			return fmt.Errorf("copy skills: %w", err)
		}
	}
	configPath := filepath.Join(sourceHome, "config.toml")
	if data, err := os.ReadFile(configPath); err == nil {
		updatedConfig := updateConfigBaseURL(string(data), newBaseURL)
		targetPath := filepath.Join(targetHome, "config.toml")
		if err := os.WriteFile(targetPath, []byte(updatedConfig), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
	} else {
		if err := writeConfigFile(targetHome, newBaseURL); err != nil {
			return err
		}
	}
	return nil
}

func updateConfigBaseURL(content, newBaseURL string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "base_url") && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				lines[i] = strings.Replace(line, trimmed, parts[0]+" = "+tomlString(newBaseURL), 1)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}
