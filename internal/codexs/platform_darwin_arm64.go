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
	return "", fmt.Errorf("terminal kind is no longer supported")
}

func (macOSAppleSiliconPlatform) OpenTerminal(kind TerminalKind, shellCommand string) error {
	// Detect current terminal application
	terminalApp := detectCurrentTerminal()

	var script string
	switch terminalApp {
	case "iTerm":
		script = "tell application \"iTerm\"\nactivate\ncreate window with default profile\ntell current session of current window to write text " + appleScriptString(shellCommand) + "\nend tell\nend tell"
	case "Terminal":
		script = "tell application \"Terminal\"\nactivate\ndo script " + appleScriptString(shellCommand) + "\nend tell"
	default:
		// Unsupported terminal detected
		return fmt.Errorf("terminal %q does not support AppleScript automation\nhint: please use Terminal.app or iTerm2, or use `codexs run` instead of `codexs open`", terminalApp)
	}

	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func detectCurrentTerminal() string {
	// Check TERM_PROGRAM environment variable
	termProgram := os.Getenv("TERM_PROGRAM")

	// Supported terminals with AppleScript integration
	if strings.Contains(termProgram, "iTerm") {
		return "iTerm"
	}
	if strings.Contains(termProgram, "Apple_Terminal") {
		return "Terminal"
	}

	// Check for known unsupported terminals
	if termProgram != "" {
		// Return the actual terminal name (e.g., "ghostty", "Alacritty", etc.)
		return termProgram
	}

	// Fallback: check parent process
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", os.Getppid()), "-o", "comm=")
	output, err := cmd.Output()
	if err == nil {
		procName := strings.TrimSpace(string(output))
		if strings.Contains(procName, "iTerm") {
			return "iTerm"
		}
		if strings.Contains(procName, "Terminal") {
			return "Terminal"
		}
		// Return the actual process name
		return filepath.Base(procName)
	}

	// Unknown terminal
	return "unknown"
}

func (macOSAppleSiliconPlatform) ExecProcess(path string, argv []string, env []string) error {
	return syscall.Exec(path, argv, env)
}

func appleScriptString(value string) string {
	return strconv.Quote(value)
}
