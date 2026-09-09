package core

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGetPackwizLocalCache(t *testing.T) {
	// Should always return a path ending in "packwiz" under the user's
	// cache dir. We don't pin the prefix — just verify the suffix and
	// the no-error contract.
	got, err := GetPackwizLocalCache()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if filepath.Base(got) != "packwiz" {
		t.Errorf("path %q should end in packwiz", got)
	}
}

func TestGetPackwizLocalStore_LinuxXDG(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_DATA_HOME branch is linux-only")
	}

	t.Setenv("XDG_DATA_HOME", "/tmp/test-xdg-data")

	got, err := GetPackwizLocalStore()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join("/tmp/test-xdg-data", "packwiz")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetPackwizInstallBinFile(t *testing.T) {
	got, err := GetPackwizInstallBinFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The leaf should be the platform-appropriate executable name.
	var wantLeaf string
	if runtime.GOOS == "windows" {
		wantLeaf = "packwiz.exe"
	} else {
		wantLeaf = "packwiz"
	}

	if filepath.Base(got) != wantLeaf {
		t.Errorf("leaf of %q = %q, want %q", got, filepath.Base(got), wantLeaf)
	}

	if !strings.Contains(got, "bin") {
		t.Errorf("path %q should contain a 'bin' segment", got)
	}
}

func TestGetPackwizCache(t *testing.T) {
	t.Cleanup(func() { viper.Set("cache.directory", nil) })

	t.Run("returns configured cache.directory when set", func(t *testing.T) {
		viper.Set("cache.directory", "/tmp/configured-cache")

		got, err := GetPackwizCache()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "/tmp/configured-cache" {
			t.Errorf("got %q, want /tmp/configured-cache", got)
		}
	})

	t.Run("falls back to local cache when unset", func(t *testing.T) {
		viper.Set("cache.directory", "")

		got, err := GetPackwizCache()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should end in packwiz/cache.
		if filepath.Base(got) != "cache" || filepath.Base(filepath.Dir(got)) != "packwiz" {
			t.Errorf("fallback path %q should end in packwiz/cache", got)
		}
	})
}
