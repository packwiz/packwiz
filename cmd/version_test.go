package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/viper"
)

// writeTestPack writes a minimal pack.toml to dir and sets viper's pack-file.
// The previous pack-file value is restored via t.Cleanup.
func writeTestPack(t *testing.T, dir string, version string) string {
	t.Helper()
	prev := viper.GetString("pack-file")
	t.Cleanup(func() {
		viper.Set("pack-file", prev)
	})

	packToml := `name = "testpack"
version = "` + version + `"
pack-format = "packwiz:1.1.0"

[index]
file = "index.toml"
hash-format = "sha256"
hash = ""
`
	path := filepath.Join(dir, "pack.toml")
	if err := os.WriteFile(path, []byte(packToml), 0644); err != nil {
		t.Fatalf("failed to write test pack.toml: %s", err)
	}
	viper.Set("pack-file", path)
	return path
}

// loadTestPack loads pack.toml from the viper pack-file path.
func loadTestPack(t *testing.T) *core.Pack {
	t.Helper()
	pack, err := core.LoadPack()
	if err != nil {
		t.Fatalf("failed to load test pack: %s", err)
	}
	return &pack
}

func TestVersionGet(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.2.3")

	pack := loadTestPack(t)
	version, err := runVersionGet(pack)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %q", version)
	}
}

func TestVersionGetNoVersion(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "")

	pack := loadTestPack(t)
	_, err := runVersionGet(pack)
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
	if !strings.Contains(err.Error(), "no version is set in pack.toml") {
		t.Errorf("unexpected error message: %s", err)
	}
}

func TestVersionSet(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.0.0")

	pack := loadTestPack(t)
	newVersion, err := runVersionSet(pack, "2.5.0")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if newVersion != "2.5.0" {
		t.Errorf("expected new version '2.5.0', got %q", newVersion)
	}

	// Verify pack.toml was actually updated
	data, err := os.ReadFile(filepath.Join(dir, "pack.toml"))
	if err != nil {
		t.Fatalf("failed to read pack.toml after set: %s", err)
	}
	if !strings.Contains(string(data), "version = \"2.5.0\"") {
		t.Errorf("expected pack.toml to contain version = \"2.5.0\", got:\n%s", string(data))
	}
}

func TestVersionSetInvalid(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.0.0")

	pack := loadTestPack(t)

	// Invalid semver
	_, err := runVersionSet(pack, "not-a-version")
	if err == nil {
		t.Fatal("expected error for invalid semver, got nil")
	}
	if !strings.Contains(err.Error(), "not a valid semantic version") {
		t.Errorf("unexpected error message: %s", err)
	}

	// Pre-release rejected
	_, err = runVersionSet(pack, "1.0.0-beta")
	if err == nil {
		t.Fatal("expected error for pre-release version, got nil")
	}

	// Build metadata rejected
	_, err = runVersionSet(pack, "1.0.0+build123")
	if err == nil {
		t.Fatal("expected error for build metadata, got nil")
	}

	// Version unchanged after all failures
	data, err := os.ReadFile(filepath.Join(dir, "pack.toml"))
	if err != nil {
		t.Fatalf("failed to read pack.toml: %s", err)
	}
	if !strings.Contains(string(data), "version = \"1.0.0\"") {
		t.Errorf("expected version to remain 1.0.0 after failed sets, got:\n%s", string(data))
	}
}

func TestVersionSetPreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	// pack.toml with fields beyond version
	packToml := `name = "testpack"
version = "1.0.0"
description = "A test modpack"
pack-format = "packwiz:1.1.0"

[index]
file = "index.toml"
hash-format = "sha256"
hash = ""
`
	path := filepath.Join(dir, "pack.toml")
	if err := os.WriteFile(path, []byte(packToml), 0644); err != nil {
		t.Fatalf("failed to write test pack.toml: %s", err)
	}
	prev := viper.GetString("pack-file")
	t.Cleanup(func() {
		viper.Set("pack-file", prev)
	})
	viper.Set("pack-file", path)

	pack := loadTestPack(t)
	newVersion, err := runVersionSet(pack, "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if newVersion != "2.0.0" {
		t.Errorf("expected new version '2.0.0', got %q", newVersion)
	}

	// Verify non-version fields survived the write
	data, err := os.ReadFile(filepath.Join(dir, "pack.toml"))
	if err != nil {
		t.Fatalf("failed to read pack.toml after set: %s", err)
	}
	content := string(data)
	if !strings.Contains(content, `name = "testpack"`) {
		t.Errorf("expected name field preserved, got:\n%s", content)
	}
	if !strings.Contains(content, `description = "A test modpack"`) {
		t.Errorf("expected description field preserved, got:\n%s", content)
	}
	if !strings.Contains(content, `version = "2.0.0"`) {
		t.Errorf("expected version = \"2.0.0\", got:\n%s", content)
	}
}

func TestVersionBumpPatch(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.0.0")

	pack := loadTestPack(t)
	previous, newVersion, err := runVersionBump(pack, "patch")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if previous != "1.0.0" || newVersion != "1.0.1" {
		t.Errorf("expected (1.0.0, 1.0.1), got (%q, %q)", previous, newVersion)
	}

	data, err := os.ReadFile(filepath.Join(dir, "pack.toml"))
	if err != nil {
		t.Fatalf("failed to read pack.toml: %s", err)
	}
	if !strings.Contains(string(data), "version = \"1.0.1\"") {
		t.Errorf("expected version = \"1.0.1\", got:\n%s", string(data))
	}
}

func TestVersionBumpMinor(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.0.0")

	pack := loadTestPack(t)
	previous, newVersion, err := runVersionBump(pack, "minor")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if previous != "1.0.0" || newVersion != "1.1.0" {
		t.Errorf("expected (1.0.0, 1.1.0), got (%q, %q)", previous, newVersion)
	}
}

func TestVersionBumpMajor(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.0.0")

	pack := loadTestPack(t)
	previous, newVersion, err := runVersionBump(pack, "major")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if previous != "1.0.0" || newVersion != "2.0.0" {
		t.Errorf("expected (1.0.0, 2.0.0), got (%q, %q)", previous, newVersion)
	}
}

func TestVersionBumpNoVersion(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "")

	pack := loadTestPack(t)
	_, _, err := runVersionBump(pack, "patch")
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
	if !strings.Contains(err.Error(), "nothing to bump") {
		t.Errorf("unexpected error message: %s", err)
	}
}

func TestVersionBumpPreReleaseRejected(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.0.0-rc1")

	pack := loadTestPack(t)
	_, _, err := runVersionBump(pack, "patch")
	if err == nil {
		t.Fatal("expected error for pre-release version, got nil")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected pre-release rejection error, got: %s", err)
	}
}

func TestVersionBumpInvalidLevel(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, dir, "1.0.0")

	pack := loadTestPack(t)
	_, _, err := runVersionBump(pack, "nano")
	if err == nil {
		t.Fatal("expected error for invalid bump level, got nil")
	}
	if !strings.Contains(err.Error(), "invalid bump level") {
		t.Errorf("unexpected error message: %s", err)
	}
}
