//go:build !darwin || !arm64

package codexs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type unsupportedPlatform struct{}

func newRuntimePlatform() Platform {
	return unsupportedPlatform{}
}

func (unsupportedPlatform) ID() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func (p unsupportedPlatform) CheckSupported() error {
	return fmt.Errorf("%s does not support %s yet; platform behavior is isolated behind the Platform interface for future implementations", appID, p.ID())
}

func (unsupportedPlatform) DefaultStoreRoot(appID string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err == nil {
		return filepath.Join(configDir, appID), nil
	}
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(home, "."+appID), nil
}

func (p unsupportedPlatform) ParseTerminalKind(value string) (TerminalKind, error) {
	if value != "" {
		return "", fmt.Errorf("terminal %q is not supported on %s yet", value, p.ID())
	}
	return "", p.CheckSupported()
}

func (p unsupportedPlatform) OpenTerminal(kind TerminalKind, shellCommand string) error {
	return p.CheckSupported()
}

func (p unsupportedPlatform) ExecProcess(path string, argv []string, env []string) error {
	return p.CheckSupported()
}
