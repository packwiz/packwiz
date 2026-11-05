package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestGetMCVersion(t *testing.T) {
	t.Run("returns version when set", func(t *testing.T) {
		p := Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

		got, err := p.GetMCVersion()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "1.20.1" {
			t.Errorf("got %q, want %q", got, "1.20.1")
		}
	})

	t.Run("returns error when minecraft version absent", func(t *testing.T) {
		p := Pack{Versions: map[string]string{"fabric": "0.15.0"}}

		if _, err := p.GetMCVersion(); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestGetSupportedMCVersions(t *testing.T) {
	// Use a clean viper state for the duration of this test.
	t.Cleanup(func() { viper.Set("acceptable-game-versions", nil) })
	viper.Set("acceptable-game-versions", nil)

	t.Run("returns just the configured version when no extras", func(t *testing.T) {
		p := Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

		got, err := p.GetSupportedMCVersions()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, []string{"1.20.1"}) {
			t.Errorf("got %v, want [1.20.1]", got)
		}
	})

	t.Run("appends acceptable-game-versions and dedups so main version is last", func(t *testing.T) {
		viper.Set("acceptable-game-versions", []string{"1.19.4", "1.20.1", "1.20"})
		p := Pack{Versions: map[string]string{"minecraft": "1.20.1"}}

		got, err := p.GetSupportedMCVersions()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 1.20.1 appears in extras AND as main; the dedup logic keeps the
		// later occurrence, so the main version ends up last.
		want := []string{"1.19.4", "1.20", "1.20.1"}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("returns error when minecraft version absent", func(t *testing.T) {
		viper.Set("acceptable-game-versions", nil)
		p := Pack{Versions: map[string]string{}}

		if _, err := p.GetSupportedMCVersions(); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestGetPackName(t *testing.T) {
	cases := []struct {
		name    string
		pack    Pack
		want    string
	}{
		{"empty name falls back to export", Pack{}, "export"},
		{"name only", Pack{Name: "MyPack"}, "MyPack"},
		{"name plus version", Pack{Name: "MyPack", Version: "1.0"}, "MyPack-1.0"},
		{"version without name still falls back", Pack{Version: "1.0"}, "export"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pack.GetPackName(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetCompatibleLoaders(t *testing.T) {
	cases := []struct {
		name     string
		versions map[string]string
		want     []string
	}{
		{"quilt implies fabric compat", map[string]string{"quilt": "1"}, []string{"quilt", "fabric"}},
		{"fabric alone", map[string]string{"fabric": "1"}, []string{"fabric"}},
		{"neoforge implies forge compat", map[string]string{"neoforge": "1"}, []string{"neoforge", "forge"}},
		{"forge alone", map[string]string{"forge": "1"}, []string{"forge"}},
		{"quilt + neoforge", map[string]string{"quilt": "1", "neoforge": "1"}, []string{"quilt", "fabric", "neoforge", "forge"}},
		{"none", map[string]string{}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Pack{Versions: tc.versions}

			got := p.GetCompatibleLoaders()

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetLoaders(t *testing.T) {
	cases := []struct {
		name     string
		versions map[string]string
		want     []string
	}{
		{"single fabric", map[string]string{"fabric": "1"}, []string{"fabric"}},
		{"quilt does NOT imply fabric here", map[string]string{"quilt": "1"}, []string{"quilt"}},
		{"all four declared", map[string]string{"quilt": "1", "fabric": "1", "neoforge": "1", "forge": "1"}, []string{"quilt", "fabric", "neoforge", "forge"}},
		{"none", map[string]string{}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Pack{Versions: tc.versions}

			got := p.GetLoaders()

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadPack_WriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	packPath := filepath.Join(dir, "pack.toml")

	t.Cleanup(func() { viper.Set("pack-file", "") })
	viper.Set("pack-file", packPath)

	original := Pack{
		Name:        "TestPack",
		Author:      "tester",
		Version:     "1.0.0",
		Description: "A pack used by TestLoadPack_WriteRoundTrip",
		PackFormat:  CurrentPackFormat,
		Versions: map[string]string{
			"minecraft": "1.20.1",
			"fabric":    "0.15.0",
		},
	}

	original.Index.File = "index.toml"
	original.Index.HashFormat = "sha256"
	original.Index.Hash = "deadbeef"

	if err := original.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	loaded, err := LoadPack()
	if err != nil {
		t.Fatalf("LoadPack: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
	}

	if loaded.Author != original.Author {
		t.Errorf("Author = %q, want %q", loaded.Author, original.Author)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, original.Version)
	}

	if loaded.PackFormat != original.PackFormat {
		t.Errorf("PackFormat = %q, want %q", loaded.PackFormat, original.PackFormat)
	}

	if loaded.Index.File != original.Index.File {
		t.Errorf("Index.File = %q, want %q", loaded.Index.File, original.Index.File)
	}

	if loaded.Index.Hash != original.Index.Hash {
		t.Errorf("Index.Hash = %q, want %q", loaded.Index.Hash, original.Index.Hash)
	}

	if !reflect.DeepEqual(loaded.Versions, original.Versions) {
		t.Errorf("Versions = %v, want %v", loaded.Versions, original.Versions)
	}
}

func TestLoadPack_AutoMigratesOldFormat(t *testing.T) {
	dir := t.TempDir()
	packPath := filepath.Join(dir, "pack.toml")

	t.Cleanup(func() { viper.Set("pack-file", "") })
	viper.Set("pack-file", packPath)

	// Pack written with the older packwiz:1.0.0 format should be
	// auto-migrated to 1.1.0 on load.
	original := Pack{
		Name:       "OldPack",
		PackFormat: "packwiz:1.0.0",
		Versions:   map[string]string{"minecraft": "1.20.1"},
	}
	original.Index.File = "index.toml"
	original.Index.HashFormat = "sha256"

	if err := original.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	loaded, err := LoadPack()
	if err != nil {
		t.Fatalf("LoadPack: %v", err)
	}

	if loaded.PackFormat != "packwiz:1.1.0" {
		t.Errorf("PackFormat = %q, want packwiz:1.1.0 (migrated)", loaded.PackFormat)
	}
}

func TestLoadPack_DefaultsIndexFile(t *testing.T) {
	dir := t.TempDir()
	packPath := filepath.Join(dir, "pack.toml")

	t.Cleanup(func() { viper.Set("pack-file", "") })
	viper.Set("pack-file", packPath)

	original := Pack{
		Name:       "DefaultIndexPack",
		PackFormat: CurrentPackFormat,
		Versions:   map[string]string{"minecraft": "1.20.1"},
	}

	if err := original.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	loaded, err := LoadPack()
	if err != nil {
		t.Fatalf("LoadPack: %v", err)
	}

	if loaded.Index.File != "index.toml" {
		t.Errorf("Index.File = %q, want index.toml (default)", loaded.Index.File)
	}
}

func TestLoadPack_RejectsNonPackwizFormat(t *testing.T) {
	dir := t.TempDir()
	packPath := filepath.Join(dir, "pack.toml")

	t.Cleanup(func() { viper.Set("pack-file", "") })
	viper.Set("pack-file", packPath)

	bad := `name = "Bad"
pack-format = "modpack:1.0.0"
[index]
file = "index.toml"
hash-format = "sha256"
[versions]
minecraft = "1.20.1"
`
	if err := os.WriteFile(packPath, []byte(bad), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := LoadPack(); err == nil {
		t.Error("expected error for non-packwiz pack-format, got nil")
	}
}
