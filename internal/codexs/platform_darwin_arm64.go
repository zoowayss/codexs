//go:build darwin && arm64

package codexs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type macOSAppleSiliconPlatform struct{}

func newRuntimePlatform() Platform {
	return macOSAppleSiliconPlatform{}
}

func (macOSAppleSiliconPlatform) ID() string {
	return "darwin/arm64"
}

func (macOSAppleSiliconPlatform) CheckSupported() error {
	return nil
}

func (macOSAppleSiliconPlatform) DefaultStoreRoot(appID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", appID), nil
}

func (macOSAppleSiliconPlatform) ParseTerminalKind(value string) (TerminalKind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(TerminalApple):
		return TerminalApple, nil
	case string(TerminalITerm), "iterm":
		return TerminalITerm, nil
	default:
		return "", fmt.Errorf("unsupported terminal %q; use terminal or iterm2", value)
	}
}

func (macOSAppleSiliconPlatform) OpenTerminal(kind TerminalKind, shellCommand string) error {
	var script string
	switch kind {
	case TerminalApple:
		script = "tell application \"Terminal\"\nactivate\ndo script " + appleScriptString(shellCommand) + "\nend tell"
	case TerminalITerm:
		script = "tell application \"iTerm2\"\nactivate\ncreate window with default profile\ntell current session of current window to write text " + appleScriptString(shellCommand) + "\nend tell"
	default:
		return fmt.Errorf("unsupported terminal %q", kind)
	}
	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (macOSAppleSiliconPlatform) ExecProcess(path string, argv []string, env []string) error {
	return syscall.Exec(path, argv, env)
}

func appleScriptString(value string) string {
	return strconv.Quote(value)
}
