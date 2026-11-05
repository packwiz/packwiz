package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlugifyName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"already-a-slug", "sodium", "sodium"},
		{"lowercases", "Sodium", "sodium"},
		{"replaces spaces with dashes", "Iris Shaders", "iris-shaders"},
		{"strips parenthesized suffix", "A Man With Plushies (AMWPlushies)", "a-man-with-plushies"},
		{"strips ' - subtitle' suffix", "Mod Name - Forge Edition", "mod-name"},
		{"collapses runs of dashes", "Foo - - Bar", "foo"},
		{"removes leading and trailing dashes", "  spaced name  ", "spaced-name"},
		{"keeps digits", "BetterF3 v4", "betterf3-v4"},
		{"drops non-alphanumeric characters", "Re:Source Pack!", "re-source-pack"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SlugifyName(tc.input)

			if got != tc.want {
				t.Errorf("SlugifyName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestModPathAccessors(t *testing.T) {
	var m Mod

	got := m.SetMetaPath(filepath.Join("mods", "sodium.pw.toml"))
	want := filepath.Join("mods", "sodium.pw.toml")

	if got != want {
		t.Errorf("SetMetaPath returned %q, want %q", got, want)
	}

	if got := m.GetFilePath(); got != want {
		t.Errorf("GetFilePath = %q, want %q", got, want)
	}

	m.FileName = "sodium-fabric-mc1.20.1-0.5.3.jar"
	gotDest := m.GetDestFilePath()
	wantDest := filepath.Join("mods", "sodium-fabric-mc1.20.1-0.5.3.jar")

	if gotDest != wantDest {
		t.Errorf("GetDestFilePath = %q, want %q", gotDest, wantDest)
	}
}

func TestGetParsedUpdateData(t *testing.T) {
	m := Mod{
		updateData: map[string]interface{}{
			"curseforge": map[string]int{"file-id": 12345},
		},
	}

	got, ok := m.GetParsedUpdateData("curseforge")
	if !ok {
		t.Fatalf("expected GetParsedUpdateData(curseforge) to find entry")
	}

	if m, isMap := got.(map[string]int); !isMap || m["file-id"] != 12345 {
		t.Errorf("got %#v, want map with file-id=12345", got)
	}

	if _, ok := m.GetParsedUpdateData("modrinth"); ok {
		t.Errorf("expected miss for modrinth, got hit")
	}
}

func TestLoadMod_WriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "sodium.pw.toml")

	original := Mod{
		Name:     "Sodium",
		FileName: "sodium-fabric-mc1.20.1-0.5.3.jar",
		Side:     "client",
		Pin:      true,
		Download: ModDownload{
			URL:        "https://cdn.modrinth.com/data/AANobbMI/versions/sodium-fabric.jar",
			HashFormat: "sha512",
			Hash:       "0123456789abcdef",
			Mode:       "",
		},
	}

	original.SetMetaPath(modPath)

	hashFormat, hashString, err := original.Write()
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if hashFormat != "sha256" {
		t.Errorf("Write returned hashFormat = %q, want sha256", hashFormat)
	}

	if hashString == "" {
		t.Error("Write returned empty hash string")
	}

	loaded, err := LoadMod(modPath)
	if err != nil {
		t.Fatalf("LoadMod: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
	}

	if loaded.FileName != original.FileName {
		t.Errorf("FileName = %q, want %q", loaded.FileName, original.FileName)
	}

	if loaded.Side != original.Side {
		t.Errorf("Side = %q, want %q", loaded.Side, original.Side)
	}

	if loaded.Pin != original.Pin {
		t.Errorf("Pin = %v, want %v", loaded.Pin, original.Pin)
	}

	if loaded.Download != original.Download {
		t.Errorf("Download = %+v, want %+v", loaded.Download, original.Download)
	}

	// metaFile is set during LoadMod from the supplied path.
	if loaded.GetFilePath() != modPath {
		t.Errorf("GetFilePath = %q, want %q", loaded.GetFilePath(), modPath)
	}
}

func TestMod_Write_CreatesContainingDirectory(t *testing.T) {
	// Write attempts MkdirAll if the parent directory is missing.
	// Verify a mod whose metaFile points at a nested path that does
	// not yet exist still succeeds.
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "deep", "nested", "mod.pw.toml")

	m := Mod{Name: "Test", FileName: "test.jar"}
	m.SetMetaPath(nestedPath)

	if _, _, err := m.Write(); err != nil {
		t.Fatalf("Write into a non-existent parent directory failed: %v", err)
	}

	if _, err := os.Stat(nestedPath); err != nil {
		t.Errorf("expected file at %q, stat error: %v", nestedPath, err)
	}
}
