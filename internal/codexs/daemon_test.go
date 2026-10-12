package codexs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type daemonStopInvocation struct {
	Args      []string
	Home      string
	HomeCount int
}

// Run the test executable as a fake Codex CLI so tests never stop real daemons.
func TestMain(m *testing.M) {
	if os.Getenv("CODEXS_TEST_DAEMON_HELPER") == "1" {
		os.Exit(runDaemonStopHelper())
	}
	os.Exit(m.Run())
}

func runDaemonStopHelper() int {
	invocation := daemonStopInvocation{Args: os.Args[1:], Home: os.Getenv("CODEX_HOME")}
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "CODEX_HOME=") {
			invocation.HomeCount++
		}
	}
	file, err := os.OpenFile(os.Getenv("CODEXS_TEST_DAEMON_LOG"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := json.NewEncoder(file).Encode(invocation); err != nil {
		fmt.Fprintln(os.Stderr, err)
		file.Close()
		return 1
	}
	if err := file.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if filepath.Base(invocation.Home) == os.Getenv("CODEXS_TEST_FAIL_PROFILE") {
		fmt.Fprintln(os.Stderr, "test daemon stop failed")
		return 1
	}
	fmt.Fprintln(os.Stdout, `{"status":"notRunning"}`)
	return 0
}

func TestCLIStopProfileDaemons(t *testing.T) {
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		profiles     []string
		args         []string
		failProfile  string
		invalidName  string
		corrupt      bool
		unsupported  error
		wantCode     int
		wantProfiles []string
		wantError    string
	}{
		{name: "only registered profiles", profiles: []string{"a-first", "b-next"}, args: []string{"stop"}, wantProfiles: []string{"a-first", "b-next"}},
		{name: "continue after failure", profiles: []string{"a-first", "b-next"}, args: []string{"stop"}, failProfile: "a-first", wantCode: 1, wantProfiles: []string{"a-first", "b-next"}, wantError: "failed to stop background servers for 1 of 2 profiles"},
		{name: "reject profile path escape", profiles: []string{"b-next"}, args: []string{"stop"}, invalidName: "../../.codex", wantCode: 1, wantProfiles: []string{"b-next"}, wantError: "failed to stop background servers for 1 of 2 profiles"},
		{name: "empty store", args: []string{"stop"}},
		{name: "reject positional arguments", profiles: []string{"work"}, args: []string{"stop", "work"}, wantCode: 2, wantError: "stop does not accept positional arguments"},
		{name: "reject flags", profiles: []string{"work"}, args: []string{"stop", "--all"}, wantCode: 2, wantError: "flag provided but not defined"},
		{name: "corrupt store", args: []string{"stop"}, corrupt: true, wantCode: 1, wantError: "parse"},
		{name: "unsupported platform", profiles: []string{"work"}, args: []string{"stop"}, unsupported: fmt.Errorf("unsupported test platform"), wantCode: 1, wantError: "unsupported test platform"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := NewStoreAt(filepath.Join(root, "custom-store"))
			preserved := make(map[string][]byte)
			for _, name := range tc.profiles {
				if _, err := store.AddProfile(name, "https://api.openai.com/v1", "test-key", time.Now()); err != nil {
					t.Fatal(err)
				}
				for filename, contents := range map[string]string{
					"auth.json":     `{"OPENAI_API_KEY":"manually-updated-test-key"}`,
					"config.toml":   "# manually edited config\n",
					"history.jsonl": "saved history\n",
				} {
					path := filepath.Join(store.CodexHome(name), filename)
					preserved[path] = []byte(contents)
					if err := os.WriteFile(path, preserved[path], 0o600); err != nil {
						t.Fatal(err)
					}
				}
				packageDir := filepath.Join(store.CodexHome(name), "packages", "app-server-daemon")
				if err := os.MkdirAll(packageDir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.invalidName != "" {
				state, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				state.Profiles["invalid"] = Profile{Name: tc.invalidName}
				if err := store.Save(state); err != nil {
					t.Fatal(err)
				}
			}
			if tc.corrupt {
				if err := os.MkdirAll(store.Root(), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(store.StatePath(), []byte("invalid JSON"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			logPath := filepath.Join(root, "daemon-calls.jsonl")
			if err := os.WriteFile(logPath, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CODEXS_CODEX_BIN", testExecutable)
			t.Setenv("CODEX_HOME", filepath.Join(root, ".codex"))
			t.Setenv("CODEXS_TEST_DAEMON_HELPER", "1")
			t.Setenv("CODEXS_TEST_DAEMON_LOG", logPath)
			t.Setenv("CODEXS_TEST_FAIL_PROFILE", tc.failProfile)
			var stdout, stderr bytes.Buffer
			cli := &CLI{
				stdout:   &stdout,
				stderr:   &stderr,
				store:    store,
				platform: testPlatform{unsupported: tc.unsupported},
			}
			if code := cli.Run(tc.args); code != tc.wantCode {
				t.Fatalf("stop exit code = %d, want %d; stderr: %s", code, tc.wantCode, stderr.String())
			}
			if tc.wantError != "" && !strings.Contains(stderr.String(), tc.wantError) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.wantError)
			}
			if tc.wantCode != 0 && strings.Contains(stdout.String(), "cleanup complete") {
				t.Fatal("failed stop reported cleanup complete")
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			var gotProfiles []string
			for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
				if len(line) == 0 {
					continue
				}
				var invocation daemonStopInvocation
				if err := json.Unmarshal(line, &invocation); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(invocation.Args, []string{"app-server", "daemon", "stop"}) {
					t.Fatalf("unexpected Codex arguments: %#v", invocation.Args)
				}
				name := filepath.Base(invocation.Home)
				if invocation.Home != store.CodexHome(name) || invocation.HomeCount != 1 {
					t.Fatalf("stop used an unexpected CODEX_HOME: %+v", invocation)
				}
				gotProfiles = append(gotProfiles, name)
			}
			if !slices.Equal(gotProfiles, tc.wantProfiles) {
				t.Fatalf("stopped profiles = %v, want %v", gotProfiles, tc.wantProfiles)
			}
			for _, name := range tc.profiles {
				packageDir := filepath.Join(store.CodexHome(name), "packages", "app-server-daemon")
				_, err := os.Stat(packageDir)
				stopped := slices.Contains(tc.wantProfiles, name) && name != tc.failProfile
				if stopped && !os.IsNotExist(err) {
					t.Fatalf("stop kept daemon package %s (stat err: %v)", packageDir, err)
				}
				if !stopped && err != nil {
					t.Fatalf("daemon package %s should be kept: %v", packageDir, err)
				}
			}
			for path, want := range preserved {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("stop modified %s", path)
				}
			}
		})
	}
}

func TestCLILaunchDaemonMode(t *testing.T) {
	codexPath := "/usr/local/bin/codex"
	sessionID := "019ea0e2-d130-7022-a3bd-e92e28e22397"
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{name: "run disables daemon", args: []string{"run", "work", "--", "-C", "/tmp/my project"}, want: []string{codexPath, "--no-daemon", "-C", "/tmp/my project"}},
		{name: "run opts into daemon", args: []string{"run", "--daemon", "work", "--", "-C", "/tmp/my project"}, want: []string{codexPath, "-C", "/tmp/my project"}},
		{name: "resume disables daemon", args: []string{"resume", sessionID}, want: []string{codexPath, "--no-daemon", "resume", sessionID}},
		{name: "resume opts into daemon", args: []string{"resume", "--daemon", sessionID}, want: []string{codexPath, "resume", sessionID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStoreAt(t.TempDir())
			if _, err := store.AddProfile("work", "https://api.openai.com/v1", "test-key", time.Now()); err != nil {
				t.Fatal(err)
			}
			sessionDir := filepath.Join(store.CodexHome("work"), "sessions", "2026", "06", "07")
			if err := os.MkdirAll(sessionDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sessionDir, "rollout-2026-06-07T12-00-00-"+sessionID+".jsonl"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CODEXS_CODEX_BIN", codexPath)
			platform := &recordingPlatform{}
			var stdout, stderr bytes.Buffer
			cli := &CLI{store: store, platform: platform, stdout: &stdout, stderr: &stderr}
			if code := cli.Run(tc.args); code != 0 {
				t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
			}
			if !slices.Equal(platform.argv, tc.want) {
				t.Fatalf("argv = %#v, want %#v", platform.argv, tc.want)
			}
		})
	}
}
