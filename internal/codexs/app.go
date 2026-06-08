package codexs

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

type CLI struct {
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	now      func() time.Time
	store    *Store
	platform Platform
}

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cli, err := newCLI(stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return cli.Run(args)
}

func newCLI(stdin io.Reader, stdout, stderr io.Writer) (*CLI, error) {
	platform := newRuntimePlatform()
	store, err := NewStoreForPlatform(platform)
	if err != nil {
		return nil, err
	}
	return &CLI{
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		now:      time.Now,
		store:    store,
		platform: platform,
	}, nil
}

func (c *CLI) Run(args []string) int {
	if len(args) == 0 {
		c.printHelp()
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		c.printHelp()
		return 0
	case "names":
		c.printNameOptions()
		return 0
	case "init":
		return c.cmdInit(args[1:])
	case "add":
		return c.cmdAdd(args[1:])
	case "update":
		return c.cmdUpdate(args[1:])
	case "list", "ls":
		return c.cmdList(args[1:])
	case "show":
		return c.cmdShow(args[1:])
	case "delete", "rm":
		return c.cmdDelete(args[1:])
	case "home":
		return c.cmdHome(args[1:])
	case "run":
		return c.cmdRun(args[1:])
	case "open":
		return c.cmdOpen(args[1:])
	case "resume":
		return c.cmdResume(args[1:])
	case "doctor":
		return c.cmdDoctor(args[1:])
	default:
		fmt.Fprintf(c.stderr, "error: unknown command %q\n\n", args[0])
		c.printHelp()
		return 2
	}
}

func (c *CLI) printHelp() {
	fmt.Fprintf(c.stdout, `codexs - Codex profile launcher for macOS Apple Silicon

Usage:
  codexs init
  codexs add <profile> --base-url <url> (--api-key <key> | --api-key-file <path> | --api-key-stdin)
  codexs update <profile> [--base-url <url>] [--api-key <key> | --api-key-file <path> | --api-key-stdin]
  codexs list
  codexs show <profile>
  codexs run <profile> [-- <codex args...>]
  codexs open <profile>... [-- <codex args...>]
  codexs resume <sessionId>
  codexs delete <profile> [--purge]
  codexs doctor
  codexs names

Examples:
  codexs add work --base-url https://api.openai.com/v1 --api-key-stdin
  codexs run work -- -C /path/to/repo
  codexs open work side-project -- -C /path/to/repo
  codexs resume 019ea0e2-d130-7022-a3bd-e92e28e22397

`)
}

func (c *CLI) printNameOptions() {
	fmt.Fprintln(c.stdout, "Project name options:")
	fmt.Fprintln(c.stdout, "  Codex Lens")
	fmt.Fprintln(c.stdout, "  Codex Profiles")
	fmt.Fprintln(c.stdout, "  Codex Dock")
	fmt.Fprintln(c.stdout, "  Codex Switchboard")
	fmt.Fprintln(c.stdout, "  Codex Persona")
	fmt.Fprintln(c.stdout, "  Codex Matrix")
	fmt.Fprintln(c.stdout, "  Codex Harbor")
	fmt.Fprintln(c.stdout, "  Codex Passport")
	fmt.Fprintln(c.stdout, "  CodeNest for Codex")
}

func (c *CLI) cmdInit(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(c.stderr, "error: init does not accept positional arguments")
		return 2
	}
	if err := c.store.Ensure(); err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "initialized %s\n", c.store.Root())
	fmt.Fprintf(c.stdout, "profiles file: %s\n", c.store.StatePath())
	return 0
}

func (c *CLI) cmdAdd(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	baseURL := fs.String("base-url", defaultBaseURL, "Codex API base URL")
	secretFlags := addSecretFlags(fs)
	profileName, flagArgs, err := extractSingleProfileArg(args, map[string]bool{
		"base-url":     true,
		"api-key":      true,
		"api-key-file": true,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 2
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(c.stderr, "error: add received unexpected positional arguments")
		return 2
	}
	apiKey, err := c.resolveSecret(secretFlags)
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 2
	}
	profile, err := c.store.AddProfile(profileName, *baseURL, apiKey, c.now())
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "added profile %q\n", profile.Name)
	fmt.Fprintf(c.stdout, "codex home: %s\n", c.store.CodexHome(profile.Name))
	return 0
}

func (c *CLI) cmdUpdate(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	baseURL := fs.String("base-url", "", "Codex API base URL")
	secretFlags := addSecretFlags(fs)
	profileName, flagArgs, err := extractSingleProfileArg(args, map[string]bool{
		"base-url":     true,
		"api-key":      true,
		"api-key-file": true,
	})
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 2
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(c.stderr, "error: update received unexpected positional arguments")
		return 2
	}
	secretSelected := secretFlags.selected()
	if *baseURL == "" && !secretSelected {
		fmt.Fprintln(c.stderr, "error: update requires --base-url or an api key option")
		return 2
	}
	var apiKey *string
	if secretSelected {
		value, err := c.resolveSecret(secretFlags)
		if err != nil {
			fmt.Fprintf(c.stderr, "error: %v\n", err)
			return 2
		}
		apiKey = &value
	}
	profile, err := c.store.UpdateProfile(profileName, *baseURL, apiKey, c.now())
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "updated profile %q\n", profile.Name)
	return 0
}

func (c *CLI) cmdList(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(c.stderr, "error: list does not accept positional arguments")
		return 2
	}
	profiles, err := c.store.ListProfiles()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	if len(profiles) == 0 {
		fmt.Fprintln(c.stdout, "no profiles")
		return 0
	}
	tw := tabwriter.NewWriter(c.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tBASE URL\tCODEX HOME")
	for _, profile := range profiles {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", profile.Name, profile.BaseURL, c.store.CodexHome(profile.Name))
	}
	tw.Flush()
	return 0
}

func (c *CLI) cmdShow(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(c.stderr, "error: show requires exactly one profile name")
		return 2
	}
	profile, err := c.store.GetProfile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "name: %s\n", profile.Name)
	fmt.Fprintf(c.stdout, "base_url: %s\n", profile.BaseURL)
	fmt.Fprintf(c.stdout, "codex_home: %s\n", c.store.CodexHome(profile.Name))
	fmt.Fprintf(c.stdout, "config: %s\n", filepath.Join(c.store.CodexHome(profile.Name), "config.toml"))
	fmt.Fprintf(c.stdout, "auth: %s\n", filepath.Join(c.store.CodexHome(profile.Name), "auth.json"))
	fmt.Fprintf(c.stdout, "created_at: %s\n", profile.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(c.stdout, "updated_at: %s\n", profile.UpdatedAt.Format(time.RFC3339))
	return 0
}

func (c *CLI) cmdDelete(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	purge := fs.Bool("purge", false, "also remove the profile CODEX_HOME files")
	profileName, flagArgs, err := extractSingleProfileArg(args, nil)
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 2
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(c.stderr, "error: delete received unexpected positional arguments")
		return 2
	}
	if err := c.store.DeleteProfile(profileName, *purge); err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	if *purge {
		fmt.Fprintf(c.stdout, "deleted profile %q and removed its files\n", profileName)
	} else {
		fmt.Fprintf(c.stdout, "deleted profile %q; profile files were left on disk\n", profileName)
	}
	return 0
}

func (c *CLI) cmdHome(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("home", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(c.stderr, "error: home requires exactly one profile name")
		return 2
	}
	profile, err := c.store.GetProfile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(c.stdout, c.store.CodexHome(profile.Name))
	return 0
}

func (c *CLI) cmdRun(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	profileName, codexArgs, err := parseSingleProfileAndCodexArgs(args)
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 2
	}
	profile, err := c.store.GetProfile(profileName)
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	if err := c.store.PrepareProfile(profile); err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	authPath := filepath.Join(c.store.CodexHome(profile.Name), "auth.json")
	if _, err := os.Stat(authPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(c.stderr, "error: missing %s; run `codexs update %s --api-key-stdin`\n", authPath, profile.Name)
			return 1
		}
		fmt.Fprintf(c.stderr, "error: inspect %s: %v\n", authPath, err)
		return 1
	}
	codexPath, err := resolveCodexBinary()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	env := withEnv(os.Environ(), "CODEX_HOME", c.store.CodexHome(profile.Name))
	argv := append([]string{codexPath}, profileConfigArgs(profile)...)
	argv = append(argv, codexArgs...)
	if err := c.platform.ExecProcess(codexPath, argv, env); err != nil {
		fmt.Fprintf(c.stderr, "error: start codex: %v\n", err)
		return 1
	}
	return 0
}

func (c *CLI) cmdOpen(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	profileNames, codexArgs, err := parseProfilesAndCodexArgs(fs.Args())
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 2
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: resolve executable: %v\n", err)
		return 1
	}
	for _, profileName := range profileNames {
		profile, err := c.store.GetProfile(profileName)
		if err != nil {
			fmt.Fprintf(c.stderr, "error: %v\n", err)
			return 1
		}
		if err := c.store.PrepareProfile(profile); err != nil {
			fmt.Fprintf(c.stderr, "error: %v\n", err)
			return 1
		}
		command := buildRunShellCommand(executable, profile.Name, codexArgs)
		if err := c.platform.OpenTerminal("", command); err != nil {
			fmt.Fprintf(c.stderr, "error: open terminal for %q: %v\n", profile.Name, err)
			return 1
		}
		fmt.Fprintf(c.stdout, "opened %q in new terminal window\n", profile.Name)
	}
	return 0
}

func (c *CLI) cmdResume(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(c.stderr, "error: resume requires exactly one session ID")
		return 2
	}
	sessionID := fs.Arg(0)
	profileName, err := c.store.FindProfileForSession(sessionID)
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	profile, err := c.store.GetProfile(profileName)
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	if err := c.store.PrepareProfile(profile); err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	authPath := filepath.Join(c.store.CodexHome(profile.Name), "auth.json")
	if _, err := os.Stat(authPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(c.stderr, "error: missing %s; run `codexs update %s --api-key-stdin`\n", authPath, profile.Name)
			return 1
		}
		fmt.Fprintf(c.stderr, "error: inspect %s: %v\n", authPath, err)
		return 1
	}
	codexPath, err := resolveCodexBinary()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	env := withEnv(os.Environ(), "CODEX_HOME", c.store.CodexHome(profile.Name))
	argv := append([]string{codexPath}, profileConfigArgs(profile)...)
	argv = append(argv, "resume", sessionID)
	fmt.Fprintf(c.stdout, "resuming session %s with profile %q\n", sessionID, profile.Name)
	if err := c.platform.ExecProcess(codexPath, argv, env); err != nil {
		fmt.Fprintf(c.stderr, "error: start codex: %v\n", err)
		return 1
	}
	return 0
}

func (c *CLI) cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(c.stderr, "error: doctor does not accept positional arguments")
		return 2
	}
	fmt.Fprintf(c.stdout, "platform: %s\n", c.platform.ID())
	if err := c.platform.CheckSupported(); err != nil {
		fmt.Fprintf(c.stdout, "supported: false\n")
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "supported: true\n")
	fmt.Fprintf(c.stdout, "store: %s\n", c.store.Root())
	fmt.Fprintf(c.stdout, "profiles: %s\n", c.store.StatePath())
	profiles, err := c.store.ListProfiles()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "profile_count: %d\n", len(profiles))
	codexPath, err := resolveCodexBinary()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(c.stdout, "codex: %s\n", codexPath)
	cmd := exec.Command(codexPath, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: codex --version failed: %v\n", err)
		if len(out) > 0 {
			fmt.Fprint(c.stderr, string(out))
		}
		return 1
	}
	fmt.Fprintf(c.stdout, "codex_version: %s", string(out))
	return 0
}

type secretFlags struct {
	apiKey      *string
	apiKeyFile  *string
	apiKeyStdin *bool
}

func addSecretFlags(fs *flag.FlagSet) secretFlags {
	return secretFlags{
		apiKey:      fs.String("api-key", "", "API key value"),
		apiKeyFile:  fs.String("api-key-file", "", "path to a file containing the API key"),
		apiKeyStdin: fs.Bool("api-key-stdin", false, "read the API key from stdin"),
	}
}

func (f secretFlags) selected() bool {
	return *f.apiKey != "" || *f.apiKeyFile != "" || *f.apiKeyStdin
}

func (c *CLI) resolveSecret(flags secretFlags) (string, error) {
	selected := 0
	if *flags.apiKey != "" {
		selected++
	}
	if *flags.apiKeyFile != "" {
		selected++
	}
	if *flags.apiKeyStdin {
		selected++
	}
	if selected != 1 {
		return "", errors.New("choose exactly one api key option")
	}
	var value string
	switch {
	case *flags.apiKey != "":
		value = *flags.apiKey
	case *flags.apiKeyFile != "":
		data, err := os.ReadFile(*flags.apiKeyFile)
		if err != nil {
			return "", fmt.Errorf("read api key file: %w", err)
		}
		value = string(data)
	case *flags.apiKeyStdin:
		data, err := io.ReadAll(c.stdin)
		if err != nil {
			return "", fmt.Errorf("read api key from stdin: %w", err)
		}
		value = string(data)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("api key is required")
	}
	return value, nil
}

func (c *CLI) requireSupportedPlatform() bool {
	if err := c.platform.CheckSupported(); err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return false
	}
	return true
}

func resolveCodexBinary() (string, error) {
	if path := os.Getenv("CODEXS_CODEX_BIN"); path != "" {
		return path, nil
	}
	path, err := exec.LookPath("codex")
	if err != nil {
		return "", errors.New("codex binary not found in PATH")
	}
	return path, nil
}

func parseSingleProfileAndCodexArgs(args []string) (string, []string, error) {
	profiles, codexArgs, err := parseProfilesAndCodexArgs(args)
	if err != nil {
		return "", nil, err
	}
	if len(profiles) != 1 {
		return "", nil, errors.New("run requires exactly one profile name")
	}
	return profiles[0], codexArgs, nil
}

func parseProfilesAndCodexArgs(args []string) ([]string, []string, error) {
	var profiles []string
	for i, arg := range args {
		if arg == "--" {
			if len(profiles) == 0 {
				return nil, nil, errors.New("at least one profile name is required before --")
			}
			return profiles, args[i+1:], nil
		}
		profiles = append(profiles, arg)
	}
	if len(profiles) == 0 {
		return nil, nil, errors.New("at least one profile name is required")
	}
	return profiles, nil, nil
}

func extractSingleProfileArg(args []string, valueFlags map[string]bool) (string, []string, error) {
	var profileName string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return "", nil, errors.New("unexpected --")
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flagArgs = append(flagArgs, arg)
			name := strings.TrimLeft(arg, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			}
			if valueFlags[name] && !strings.Contains(arg, "=") {
				i++
				if i >= len(args) {
					flagArgs = append(flagArgs, "")
					continue
				}
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		if profileName != "" {
			return "", nil, fmt.Errorf("unexpected positional argument %q", arg)
		}
		profileName = arg
	}
	if profileName == "" {
		return "", nil, errors.New("exactly one profile name is required")
	}
	return profileName, flagArgs, nil
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		out = append(out, item)
	}
	return append(out, prefix+value)
}

func profileConfigArgs(profile Profile) []string {
	return []string{
		"-c", `cli_auth_credentials_store="file"`,
		"-c", `model_provider="profile"`,
		"-c", `model_providers.profile.name="profile"`,
		"-c", `model_providers.profile.wire_api="responses"`,
		"-c", "model_providers.profile.base_url=" + tomlString(profile.BaseURL),
		"-c", "model_providers.profile.requires_openai_auth=true",
	}
}
