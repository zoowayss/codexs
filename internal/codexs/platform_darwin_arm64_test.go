//go:build darwin && arm64

package codexs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMacOSAppleSiliconDefaultStoreRoot(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home directory: %v", err)
	}

	root, err := (macOSAppleSiliconPlatform{}).DefaultStoreRoot(appID)
	if err != nil {
		t.Fatalf("DefaultStoreRoot returned error: %v", err)
	}

	want := filepath.Join(home, ".codexs")
	if root != want {
		t.Fatalf("default store root = %q, want %q", root, want)
	}
}
