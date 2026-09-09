package core

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// passthroughUpdater satisfies the Updater interface for tests that
// need to load mod files referencing update plugins. ParseUpdate
// returns the input unchanged; the other methods panic because they
// should never be called by these tests.
type passthroughUpdater struct{}

func (passthroughUpdater) ParseUpdate(in map[string]interface{}) (interface{}, error) {
	return in, nil
}

func (passthroughUpdater) CheckUpdate([]*Mod, Pack) ([]UpdateCheck, error) {
	panic("passthroughUpdater.CheckUpdate should not be called in tests")
}

func (passthroughUpdater) DoUpdate([]*Mod, []interface{}) error {
	panic("passthroughUpdater.DoUpdate should not be called in tests")
}

// registerTestUpdaters installs passthrough updaters for "modrinth"
// and "curseforge" — the two backends referenced by mods in the
// vendored example pack — and restores prior state at test cleanup so
// the global Updaters map isn't polluted across tests.
func registerTestUpdaters(t *testing.T) {
	t.Helper()
	for _, name := range []string{"modrinth", "curseforge"} {
		name := name
		prev, existed := Updaters[name]
		Updaters[name] = passthroughUpdater{}

		t.Cleanup(func() {
			if existed {
				Updaters[name] = prev
			} else {
				delete(Updaters, name)
			}
		})
	}
}

// copyExamplePack replicates the vendored example pack into a temp
// directory and returns its root path. Tests that mutate the pack
// (Refresh, Write) get a fresh isolated copy each invocation.
func copyExamplePack(t *testing.T) string {
	t.Helper()

	dst := t.TempDir()
	src := filepath.Join("testdata", "example-pack")

	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	})

	if err != nil {
		t.Fatalf("copyExamplePack: %v", err)
	}

	return dst
}

func TestPack_UpdateIndexHash_FromExamplePack(t *testing.T) {
	packRoot := copyExamplePack(t)
	packPath := filepath.Join(packRoot, "pack.toml")

	t.Cleanup(func() {
		viper.Set("pack-file", "")
		viper.Set("no-internal-hashes", false)
	})
	viper.Set("pack-file", packPath)
	viper.Set("no-internal-hashes", false)

	pack, err := LoadPack()
	if err != nil {
		t.Fatalf("LoadPack: %v", err)
	}

	// Clear the hash that came from the vendored pack.toml so we can
	// verify UpdateIndexHash recomputes it correctly.
	pack.Index.Hash = ""

	if err := pack.UpdateIndexHash(); err != nil {
		t.Fatalf("UpdateIndexHash: %v", err)
	}

	// This is the value recorded in the vendored pack.toml. If
	// UpdateIndexHash produces something different, either our hash
	// pipeline regressed or the vendored fixture diverged from
	// upstream.
	const expected = "29bcf953b90081730cd8a32cc4d855ab6ad96685c3aeea6ed033dc4f04d390ac"

	if pack.Index.Hash != expected {
		t.Errorf("Index.Hash = %q, want %q", pack.Index.Hash, expected)
	}

	if pack.Index.HashFormat != "sha256" {
		t.Errorf("Index.HashFormat = %q, want sha256", pack.Index.HashFormat)
	}
}

func TestIndex_LoadAllMods_FromExamplePack(t *testing.T) {
	registerTestUpdaters(t)

	packRoot := copyExamplePack(t)
	indexPath := filepath.Join(packRoot, "index.toml")

	index, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	mods, err := index.LoadAllMods()
	if err != nil {
		t.Fatalf("LoadAllMods: %v", err)
	}

	if len(mods) != 2 {
		t.Fatalf("loaded %d mods, want 2", len(mods))
	}

	names := map[string]bool{}
	for _, m := range mods {
		names[m.Name] = true
	}

	for _, want := range []string{"Borderless Mining", "Screenshot to Clipboard (Fabric)"} {
		if !names[want] {
			t.Errorf("expected mod %q not loaded; got %v", want, names)
		}
	}

	// The two example mods reference different backends; verify
	// GetParsedUpdateData was populated for each.
	for _, m := range mods {
		var key string
		if m.Name == "Borderless Mining" {
			key = "modrinth"
		} else {
			key = "curseforge"
		}

		if _, ok := m.GetParsedUpdateData(key); !ok {
			t.Errorf("mod %q has no parsed update data for %q", m.Name, key)
		}
	}
}

func TestIndex_Refresh_FromExamplePack(t *testing.T) {
	packRoot := copyExamplePack(t)
	packPath := filepath.Join(packRoot, "pack.toml")
	indexPath := filepath.Join(packRoot, "index.toml")

	t.Cleanup(func() {
		viper.Set("pack-file", "")
		viper.Set("no-internal-hashes", false)
	})
	viper.Set("pack-file", packPath)
	viper.Set("no-internal-hashes", false)

	index, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	// Capture pre-refresh hashes for the three expected entries.
	before := map[string]string{}
	for path, holder := range index.Files {
		f, ok := holder.(*indexFile)
		if !ok {
			t.Fatalf("unexpected holder type for %q: %T", path, holder)
		}
		before[path] = f.Hash
	}

	if len(before) != 3 {
		t.Fatalf("expected 3 entries in vendored index, got %d", len(before))
	}

	if err := index.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if len(index.Files) != 3 {
		t.Errorf("expected 3 entries after refresh, got %d", len(index.Files))
	}

	for path, wantHash := range before {
		holder, ok := index.Files[path]
		if !ok {
			t.Errorf("entry %q missing after refresh", path)
			continue
		}

		f, ok := holder.(*indexFile)
		if !ok {
			t.Errorf("entry %q has unexpected holder type %T", path, holder)
			continue
		}

		if f.Hash != wantHash {
			t.Errorf("entry %q hash changed across refresh: %q → %q", path, wantHash, f.Hash)
		}
	}
}
