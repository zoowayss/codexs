package codexs

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (c *CLI) cmdStop(args []string) int {
	if !c.requireSupportedPlatform() {
		return 1
	}
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(c.stderr, "error: stop does not accept positional arguments")
		return 2
	}
	profiles, err := c.store.ListProfiles()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}
	if len(profiles) == 0 {
		fmt.Fprintln(c.stdout, "no profiles; no background servers to stop")
		return 0
	}
	codexPath, err := resolveCodexBinary()
	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		return 1
	}

	failed := 0
	for _, profile := range profiles {
		if err := validateProfileName(profile.Name); err != nil {
			fmt.Fprintf(c.stderr, "error: stop background server: %v\n", err)
			failed++
			continue
		}
		codexHome := c.store.CodexHome(profile.Name)
		fmt.Fprintf(c.stdout, "stopping Codex background server for profile %q (%s)\n", profile.Name, codexHome)
		cmd := exec.Command(codexPath, "app-server", "daemon", "stop")
		cmd.Env = withEnv(os.Environ(), "CODEX_HOME", codexHome)
		cmd.Stdout = c.stdout
		cmd.Stderr = c.stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(c.stderr, "error: stop background server for profile %q: %v\n", profile.Name, err)
			failed++
			continue
		}
		packageDir := filepath.Join(codexHome, "packages", "app-server-daemon")
		if err := os.RemoveAll(packageDir); err != nil {
			fmt.Fprintf(c.stderr, "error: remove daemon package for profile %q: %v\n", profile.Name, err)
			failed++
			continue
		}
		fmt.Fprintf(c.stdout, "removed %s\n", packageDir)
	}
	if failed > 0 {
		fmt.Fprintf(c.stderr, "failed to stop background servers for %d of %d profiles\n", failed, len(profiles))
		return 1
	}
	fmt.Fprintln(c.stdout, "profile background server cleanup complete")
	return 0
}
