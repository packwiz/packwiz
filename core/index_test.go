package core

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestIndex builds an Index rooted at packRoot with no files.
// Tests can populate Files directly before exercising methods.
func newTestIndex(packRoot string) Index {
	return Index{
		HashFormat: "sha256",
		Files:      IndexFiles{},
		indexFile:  filepath.Join(packRoot, "index.toml"),
		packRoot:   packRoot,
	}
}

func TestResolveIndexPath(t *testing.T) {
	in := newTestIndex(filepath.Join("some", "pack"))

	// Index paths are stored forward-slashed; ResolveIndexPath converts
	// to OS-native and joins with packRoot.
	got := in.ResolveIndexPath("mods/sodium.pw.toml")
	want := filepath.Join("some", "pack", "mods", "sodium.pw.toml")

	if got != want {
		t.Errorf("ResolveIndexPath = %q, want %q", got, want)
	}
}

func TestRelIndexPath(t *testing.T) {
	root := filepath.Join("some", "pack")
	in := newTestIndex(root)

	got, err := in.RelIndexPath(filepath.Join(root, "mods", "sodium.pw.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "mods/sodium.pw.toml" {
		t.Errorf("RelIndexPath = %q, want %q", got, "mods/sodium.pw.toml")
	}
}

func TestFindMod(t *testing.T) {
	root := filepath.Join("some", "pack")
	in := newTestIndex(root)
	in.Files["mods/sodium.pw.toml"] = &indexFile{File: "mods/sodium.pw.toml", MetaFile: true, fileFound: true}
	in.Files["mods/iris.pw.toml"] = &indexFile{File: "mods/iris.pw.toml", MetaFile: true, fileFound: true}
	in.Files["overrides/config.txt"] = &indexFile{File: "overrides/config.txt", fileFound: true}

	t.Run("finds a metafile by slug", func(t *testing.T) {
		got, ok := in.FindMod("sodium")
		if !ok {
			t.Fatal("expected to find sodium")
		}

		want := filepath.Join(root, "mods", "sodium.pw.toml")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("misses unknown slug", func(t *testing.T) {
		if _, ok := in.FindMod("does-not-exist"); ok {
			t.Error("expected miss for unknown slug")
		}
	})

	t.Run("skips non-metafile entries even if name matches", func(t *testing.T) {
		// "config" would match overrides/config.txt's filename minus
		// extension, but it is not a metafile and so should not be
		// returned.
		if _, ok := in.FindMod("config"); ok {
			t.Error("expected miss for non-metafile entry")
		}
	})
}

func TestRemoveFile(t *testing.T) {
	root := filepath.Join("some", "pack")
	in := newTestIndex(root)
	in.Files["mods/sodium.pw.toml"] = &indexFile{File: "mods/sodium.pw.toml"}

	err := in.RemoveFile(filepath.Join(root, "mods", "sodium.pw.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, present := in.Files["mods/sodium.pw.toml"]; present {
		t.Error("entry was not removed")
	}
}

func TestRefreshFileWithHash(t *testing.T) {
	root := filepath.Join("some", "pack")
	in := newTestIndex(root)

	err := in.RefreshFileWithHash(
		filepath.Join(root, "mods", "sodium.pw.toml"),
		"sha256", "abc123", true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	holder, ok := in.Files["mods/sodium.pw.toml"]
	if !ok {
		t.Fatal("entry was not created")
	}

	if !holder.IsMetaFile() {
		t.Error("entry should be marked as metafile")
	}

	f := holder.(*indexFile)
	// updateFileHashGiven blanks the format when it matches the index hash format.
	if f.Hash != "abc123" || f.HashFormat != "" {
		t.Errorf("got hash=%q format=%q, want hash=abc123 format=\"\"", f.Hash, f.HashFormat)
	}
}

func TestLoadIndex_WriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.toml")

	original := Index{
		HashFormat: "sha256",
		Files: IndexFiles{
			"mods/sodium.pw.toml": &indexFile{
				File:     "mods/sodium.pw.toml",
				Hash:     "abc123",
				MetaFile: true,
			},
			"mods/iris.pw.toml": &indexFile{
				File:     "mods/iris.pw.toml",
				Hash:     "def456",
				MetaFile: true,
			},
			"overrides/config.txt": &indexFile{
				File:       "overrides/config.txt",
				Hash:       "ghi789",
				HashFormat: "md5",
			},
		},
		indexFile: indexPath,
		packRoot:  dir,
	}

	if err := original.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	loaded, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	if loaded.HashFormat != "sha256" {
		t.Errorf("HashFormat = %q, want sha256", loaded.HashFormat)
	}

	if len(loaded.Files) != 3 {
		t.Fatalf("Files count = %d, want 3", len(loaded.Files))
	}

	checkEntry := func(path, wantHash, wantFormat string, wantMeta bool) {
		t.Helper()

		holder, ok := loaded.Files[path]
		if !ok {
			t.Errorf("entry %q missing from loaded index", path)
			return
		}

		f, ok := holder.(*indexFile)
		if !ok {
			t.Errorf("entry %q has unexpected holder type %T", path, holder)
			return
		}

		if f.Hash != wantHash {
			t.Errorf("entry %q hash = %q, want %q", path, f.Hash, wantHash)
		}

		if f.HashFormat != wantFormat {
			t.Errorf("entry %q hashFormat = %q, want %q", path, f.HashFormat, wantFormat)
		}

		if f.MetaFile != wantMeta {
			t.Errorf("entry %q metafile = %v, want %v", path, f.MetaFile, wantMeta)
		}
	}

	// Metafiles inherit the index's HashFormat (sha256) and so the
	// per-entry hash-format field is empty in the file representation.
	checkEntry("mods/sodium.pw.toml", "abc123", "", true)
	checkEntry("mods/iris.pw.toml", "def456", "", true)
	// Non-default hash format (md5) is preserved per-entry.
	checkEntry("overrides/config.txt", "ghi789", "md5", false)
}

func TestLoadIndex_DefaultsHashFormat(t *testing.T) {
	// If hash-format is omitted, LoadIndex should default to sha256.
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.toml")

	contents := `[[files]]
file = "mods/sodium.pw.toml"
hash = "abc"
metafile = true
`
	if err := os.WriteFile(indexPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	if loaded.HashFormat != "sha256" {
		t.Errorf("HashFormat = %q, want sha256 (default)", loaded.HashFormat)
	}
}

func TestLoadIndex_MultipleAliasRoundTrip(t *testing.T) {
	// Two file entries with the same path but different aliases should
	// coalesce into an indexFileMultipleAlias on load.
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.toml")

	contents := `hash-format = "sha256"

[[files]]
file = "mods/dual.jar"
hash = "h1"

[[files]]
file = "mods/dual.jar"
hash = "h2"
alias = "alt/dual.jar"
`
	if err := os.WriteFile(indexPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	holder, ok := loaded.Files["mods/dual.jar"]
	if !ok {
		t.Fatal("mods/dual.jar missing from loaded index")
	}

	if _, ok := holder.(*indexFileMultipleAlias); !ok {
		t.Errorf("expected *indexFileMultipleAlias holder, got %T", holder)
	}
}
