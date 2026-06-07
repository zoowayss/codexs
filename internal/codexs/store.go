package codexs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Store struct {
	root string
}

func NewStore() (*Store, error) {
	return NewStoreForPlatform(newRuntimePlatform())
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
		return Profile{}, fmt.Errorf("profile %q already exists; use update to change it", name)
	}
	profile := Profile{
		Name:      name,
		BaseURL:   normalizedBaseURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.writeProfileFiles(profile, apiKey); err != nil {
		return Profile{}, err
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
		return Profile{}, fmt.Errorf("profile %q does not exist", name)
	}
	if baseURL != "" {
		normalizedBaseURL, err := normalizeBaseURL(baseURL)
		if err != nil {
			return Profile{}, err
		}
		profile.BaseURL = normalizedBaseURL
	}
	var key string
	if apiKey != nil {
		if *apiKey == "" {
			return Profile{}, errors.New("api key is required when updating api key")
		}
		key = *apiKey
	}
	profile.UpdatedAt = now
	if err := s.writeProfileFiles(profile, key); err != nil {
		return Profile{}, err
	}
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
		return fmt.Errorf("profile %q does not exist", name)
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
		return Profile{}, fmt.Errorf("profile %q does not exist", name)
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
	return s.writeProfileFiles(profile, "")
}

func (s *Store) writeProfileFiles(profile Profile, apiKey string) error {
	codexHome := s.CodexHome(profile.Name)
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", codexHome, err)
	}
	configPath := filepath.Join(codexHome, "config.toml")
	if err := os.WriteFile(configPath, []byte(renderCodexConfig(profile.BaseURL)), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	if apiKey != "" {
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
	}
	return nil
}
