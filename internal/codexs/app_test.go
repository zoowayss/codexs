package codexs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateProfileName(t *testing.T) {
	valid := []string{"work", "team-a", "team_a", "team.a", "A1"}
	for _, name := range valid {
		if err := validateProfileName(name); err != nil {
			t.Fatalf("validateProfileName(%q) returned error: %v", name, err)
		}
	}
	invalid := []string{"", "../work", "-work", "work/a", "work a"}
	for _, name := range invalid {
		if err := validateProfileName(name); err == nil {
			t.Fatalf("validateProfileName(%q) returned nil", name)
		}
	}
}

func TestParseProfilesAndCodexArgs(t *testing.T) {
	profiles, codexArgs, err := parseProfilesAndCodexArgs([]string{"work", "side", "--", "-C", "/tmp/repo"})
	if err != nil {
		t.Fatalf("parseProfilesAndCodexArgs returned error: %v", err)
	}
	if strings.Join(profiles, ",") != "work,side" {
		t.Fatalf("profiles = %v", profiles)
	}
	if strings.Join(codexArgs, ",") != "-C,/tmp/repo" {
		t.Fatalf("codexArgs = %v", codexArgs)
	}
}

func TestStoreAddProfileWritesCodexHome(t *testing.T) {
	root := t.TempDir()
	store := NewStoreAt(root)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	profile, err := store.AddProfile("work", "https://api.openai.com/v1", "test-key", now)
	if err != nil {
		t.Fatalf("AddProfile returned error: %v", err)
	}
	if profile.Name != "work" {
		t.Fatalf("profile name = %q", profile.Name)
	}
	configPath := filepath.Join(store.CodexHome("work"), "config.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	configText := string(config)
	for _, want := range []string{
		`cli_auth_credentials_store = "file"`,
		`model_provider = "profile"`,
		`base_url = "https://api.openai.com/v1"`,
		`requires_openai_auth = true`,
	} {
		if !strings.Contains(configText, want) {
			t.Fatalf("config.toml missing %q:\n%s", want, configText)
		}
	}
	authPath := filepath.Join(store.CodexHome("work"), "auth.json")
	authData, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("read auth.json: %v", err)
	}
	var auth map[string]string
	if err := json.Unmarshal(authData, &auth); err != nil {
		t.Fatalf("parse auth.json: %v", err)
	}
	if auth["OPENAI_API_KEY"] != "test-key" {
		t.Fatalf("OPENAI_API_KEY = %q", auth["OPENAI_API_KEY"])
	}
}

func TestBuildRunShellCommandQuotesArgs(t *testing.T) {
	got := buildRunShellCommand("/Applications/My Tool/codexs", "team'a", []string{"-C", "/tmp/has space"})
	want := "'/Applications/My Tool/codexs' run 'team'\\''a' -- '-C' '/tmp/has space'"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestProfileConfigArgs(t *testing.T) {
	args := profileConfigArgs(Profile{Name: "work", BaseURL: "https://example.com/v1"})
	joined := strings.Join(args, "\n")
	for _, want := range []string{
		`cli_auth_credentials_store="file"`,
		`model_provider="profile"`,
		`model_providers.profile.base_url="https://example.com/v1"`,
		`model_providers.profile.requires_openai_auth=true`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("profileConfigArgs missing %q in %#v", want, args)
		}
	}
}

func TestNewStoreForPlatformUsesPlatformDefaultRoot(t *testing.T) {
	t.Setenv("CODEXS_HOME", "")
	root := t.TempDir()
	store, err := NewStoreForPlatform(testPlatform{id: "test/os", root: root})
	if err != nil {
		t.Fatalf("NewStoreForPlatform returned error: %v", err)
	}
	if store.Root() != root {
		t.Fatalf("store root = %q, want %q", store.Root(), root)
	}
}

func TestCLIUsesPlatformInterfaceForSupportCheck(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	unsupportedErr := errors.New("test platform is unsupported")
	cli := &CLI{
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		now:      time.Now,
		store:    NewStoreAt(t.TempDir()),
		platform: testPlatform{id: "test/os", root: t.TempDir(), unsupported: unsupportedErr},
	}

	if code := cli.Run([]string{"names"}); code != 0 {
		t.Fatalf("names exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Codex Profiles") {
		t.Fatalf("names output missing project names: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"init"}); code != 1 {
		t.Fatalf("init exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), unsupportedErr.Error()) {
		t.Fatalf("stderr missing unsupported error: %s", stderr.String())
	}
}

type testPlatform struct {
	id          string
	root        string
	unsupported error
}

func (p testPlatform) ID() string {
	return p.id
}

func (p testPlatform) CheckSupported() error {
	return p.unsupported
}

func (p testPlatform) DefaultStoreRoot(appID string) (string, error) {
	return p.root, nil
}

func (p testPlatform) OpenTerminal(shellCommand string) error {
	return p.unsupported
}

func (p testPlatform) ExecProcess(path string, argv []string, env []string) error {
	return p.unsupported
}

func TestStoreFindProfileForSession(t *testing.T) {
	root := t.TempDir()
	store := NewStoreAt(root)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	// Add a profile
	_, err := store.AddProfile("work", "https://api.openai.com/v1", "test-key", now)
	if err != nil {
		t.Fatalf("AddProfile returned error: %v", err)
	}

	// Create a fake session file
	sessionID := "019ea0e2-d130-7022-a3bd-e92e28e22397"
	sessionsDir := filepath.Join(store.CodexHome("work"), "sessions", "2026", "06", "07")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	sessionFile := filepath.Join(sessionsDir, "rollout-2026-06-07T12-00-00-"+sessionID+".jsonl")
	if err := os.WriteFile(sessionFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	// Verify session can be found
	profileName, err := store.FindProfileForSession(sessionID)
	if err != nil {
		t.Fatalf("FindProfileForSession returned error: %v", err)
	}
	if profileName != "work" {
		t.Fatalf("FindProfileForSession returned %q, want %q", profileName, "work")
	}

	// Verify session not found for non-existent session
	_, err = store.FindProfileForSession("nonexistent-session-id")
	if err == nil {
		t.Fatal("FindProfileForSession should return error for non-existent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("FindProfileForSession error = %v, want 'not found'", err)
	}
}

func TestStoreFindProfileForSessionWithCache(t *testing.T) {
	root := t.TempDir()
	store := NewStoreAt(root)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	// Add a profile
	_, err := store.AddProfile("work", "https://api.openai.com/v1", "test-key", now)
	if err != nil {
		t.Fatalf("AddProfile returned error: %v", err)
	}

	// Create a fake session file
	sessionID := "019ea0e2-d130-7022-a3bd-e92e28e22397"
	sessionsDir := filepath.Join(store.CodexHome("work"), "sessions", "2026", "06", "07")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	sessionFile := filepath.Join(sessionsDir, "rollout-2026-06-07T12-00-00-"+sessionID+".jsonl")
	if err := os.WriteFile(sessionFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	// First call - should scan and populate cache
	profileName, err := store.FindProfileForSession(sessionID)
	if err != nil {
		t.Fatalf("FindProfileForSession returned error: %v", err)
	}
	if profileName != "work" {
		t.Fatalf("FindProfileForSession returned %q, want %q", profileName, "work")
	}

	// Verify cache was populated
	cache, err := store.LoadSessionCache()
	if err != nil {
		t.Fatalf("LoadSessionCache returned error: %v", err)
	}
	if cachedProfile, ok := cache.Sessions[sessionID]; !ok || cachedProfile.ProfileName != "work" {
		t.Fatalf("cache.Sessions[%q] = %+v, want ProfileName=%q", sessionID, cachedProfile, "work")
	}

	// Second call - should use cache (we can verify by deleting the session file)
	if err := os.Remove(sessionFile); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	// Should still find it from cache
	profileName, err = store.FindProfileForSession(sessionID)
	if err != nil {
		t.Fatalf("FindProfileForSession (from cache) returned error: %v", err)
	}
	if profileName != "work" {
		t.Fatalf("FindProfileForSession (from cache) returned %q, want %q", profileName, "work")
	}
}

// Benchmark tests
func BenchmarkValidateProfileName(b *testing.B) {
	validNames := []string{"work", "team-a", "team_a", "project.prod", "A1B2"}
	for i := 0; i < b.N; i++ {
		for _, name := range validNames {
			_ = validateProfileName(name)
		}
	}
}

func BenchmarkParseProfilesAndCodexArgs(b *testing.B) {
	args := []string{"work", "side", "prod", "--", "-C", "/tmp/repo", "-v"}
	for i := 0; i < b.N; i++ {
		_, _, _ = parseProfilesAndCodexArgs(args)
	}
}

func BenchmarkBuildRunShellCommand(b *testing.B) {
	path := "/usr/local/bin/codexs"
	profile := "work"
	args := []string{"-C", "/home/user/project", "--verbose"}
	for i := 0; i < b.N; i++ {
		_ = buildRunShellCommand(path, profile, args)
	}
}

func BenchmarkStoreAddProfile(b *testing.B) {
	root := b.TempDir()
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store := NewStoreAt(filepath.Join(root, fmt.Sprintf("bench_%d", i)))
		_, _ = store.AddProfile("work", "https://api.openai.com/v1", "test-key", now)
	}
}

func BenchmarkStoreFindProfileForSession(b *testing.B) {
	root := b.TempDir()
	store := NewStoreAt(root)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	// Setup
	_, _ = store.AddProfile("work", "https://api.openai.com/v1", "test-key", now)
	sessionID := "019ea0e2-d130-7022-a3bd-e92e28e22397"
	sessionsDir := filepath.Join(store.CodexHome("work"), "sessions", "2026", "06", "07")
	_ = os.MkdirAll(sessionsDir, 0o755)
	sessionFile := filepath.Join(sessionsDir, "rollout-2026-06-07T12-00-00-"+sessionID+".jsonl")
	_ = os.WriteFile(sessionFile, []byte("test"), 0o644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.FindProfileForSession(sessionID)
	}
}

func TestStoreAddProfileFromTemplate(t *testing.T) {
	root := t.TempDir()
	store := NewStoreAt(root)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	// Create source profile
	_, err := store.AddProfile("work", "https://api.example.com/v1", "source-key", now)
	if err != nil {
		t.Fatalf("AddProfile returned error: %v", err)
	}

	// Add AGENTS.md to source profile
	agentsPath := filepath.Join(store.CodexHome("work"), "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# Test AGENTS.md\nTest content"), 0o644); err != nil {
		t.Fatalf("WriteFile AGENTS.md: %v", err)
	}

	// Add skills directory to source profile
	skillsPath := filepath.Join(store.CodexHome("work"), "skills", "test-skill")
	if err := os.MkdirAll(skillsPath, 0o755); err != nil {
		t.Fatalf("MkdirAll skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsPath, "skill.md"), []byte("test skill"), 0o644); err != nil {
		t.Fatalf("WriteFile skill.md: %v", err)
	}

	// Create new profile from template
	profile, err := store.AddProfileFromTemplate("prework", "https://newapi.example.com/v1", "new-key", "work", now)
	if err != nil {
		t.Fatalf("AddProfileFromTemplate returned error: %v", err)
	}
	if profile.Name != "prework" {
		t.Fatalf("profile name = %q", profile.Name)
	}
	if profile.BaseURL != "https://newapi.example.com/v1" {
		t.Fatalf("profile base_url = %q", profile.BaseURL)
	}

	// Verify AGENTS.md was copied
	newAgentsPath := filepath.Join(store.CodexHome("prework"), "AGENTS.md")
	agentsData, err := os.ReadFile(newAgentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsData), "Test AGENTS.md") {
		t.Fatalf("AGENTS.md content = %q", string(agentsData))
	}

	// Verify skills directory was copied
	newSkillPath := filepath.Join(store.CodexHome("prework"), "skills", "test-skill", "skill.md")
	skillData, err := os.ReadFile(newSkillPath)
	if err != nil {
		t.Fatalf("read skill.md: %v", err)
	}
	if string(skillData) != "test skill" {
		t.Fatalf("skill.md content = %q", string(skillData))
	}

	// Verify config.toml has updated base_url
	configPath := filepath.Join(store.CodexHome("prework"), "config.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, `base_url  = "https://newapi.example.com/v1"`) &&
		!strings.Contains(configText, `base_url = "https://newapi.example.com/v1"`) {
		t.Fatalf("config.toml missing updated base_url:\n%s", configText)
	}
	if strings.Contains(configText, "https://api.example.com/v1") {
		t.Fatalf("config.toml still contains old base_url:\n%s", configText)
	}

	// Verify auth.json has new API key
	authPath := filepath.Join(store.CodexHome("prework"), "auth.json")
	authData, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("read auth.json: %v", err)
	}
	var auth map[string]string
	if err := json.Unmarshal(authData, &auth); err != nil {
		t.Fatalf("parse auth.json: %v", err)
	}
	if auth["OPENAI_API_KEY"] != "new-key" {
		t.Fatalf("OPENAI_API_KEY = %q", auth["OPENAI_API_KEY"])
	}
}

func TestStoreAddProfileFromTemplateNonexistentSource(t *testing.T) {
	root := t.TempDir()
	store := NewStoreAt(root)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	_, err := store.AddProfileFromTemplate("new", "https://api.com/v1", "key", "nonexistent", now)
	if err == nil {
		t.Fatal("AddProfileFromTemplate should return error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("error = %v, want 'does not exist'", err)
	}
}
