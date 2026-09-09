package core

import (
	"reflect"
	"testing"
)

func TestIndexFileStateMachine(t *testing.T) {
	var f indexFile

	if f.IsMetaFile() {
		t.Error("zero indexFile should not be a metafile")
	}

	if f.markedFound() {
		t.Error("zero indexFile should not be markedFound")
	}

	f.markFound()
	if !f.markedFound() {
		t.Error("after markFound, markedFound should be true")
	}

	f.markMetaFile()
	if !f.IsMetaFile() {
		t.Error("after markMetaFile, IsMetaFile should be true")
	}

	f.updateHash("deadbeef", "sha256")
	if f.Hash != "deadbeef" || f.HashFormat != "sha256" {
		t.Errorf("updateHash didn't set fields: got (%q, %q)", f.Hash, f.HashFormat)
	}
}

func TestIndexFileMultipleAliasStateMachine(t *testing.T) {
	m := indexFileMultipleAlias{
		"a": {File: "mods/x.jar"},
		"b": {File: "mods/x.jar", Alias: "alt/x.jar"},
	}

	if m.IsMetaFile() {
		t.Error("expected IsMetaFile=false before markMetaFile")
	}

	if m.markedFound() {
		t.Error("expected markedFound=false before markFound")
	}

	m.markFound()
	if !m.markedFound() {
		t.Error("expected markedFound=true after markFound")
	}

	m.markMetaFile()
	if !m.IsMetaFile() {
		t.Error("expected IsMetaFile=true after markMetaFile")
	}

	m.updateHash("deadbeef", "sha256")
	for k, v := range m {
		if v.Hash != "deadbeef" || v.HashFormat != "sha256" {
			t.Errorf("entry %q has hash (%q, %q), want (deadbeef, sha256)", k, v.Hash, v.HashFormat)
		}
	}
}

func TestUpdateFileEntry(t *testing.T) {
	t.Run("adds entry when path absent", func(t *testing.T) {
		var files IndexFiles
		files.updateFileEntry("mods/sodium.pw.toml", "sha256", "abc", true)

		holder, ok := files["mods/sodium.pw.toml"]
		if !ok {
			t.Fatal("entry was not added")
		}

		if !holder.IsMetaFile() || !holder.markedFound() {
			t.Error("new entry should be marked metafile + found")
		}
	})

	t.Run("updates hash on existing entry", func(t *testing.T) {
		files := IndexFiles{}
		files.updateFileEntry("mods/x.jar", "sha256", "first", false)
		files.updateFileEntry("mods/x.jar", "sha256", "second", false)

		f := files["mods/x.jar"].(*indexFile)
		if f.Hash != "second" {
			t.Errorf("hash = %q, want %q", f.Hash, "second")
		}
	})

	t.Run("never un-sets metafile flag on existing entry", func(t *testing.T) {
		files := IndexFiles{}
		files.updateFileEntry("mods/x.pw.toml", "sha256", "abc", true)
		// Second update without markAsMetaFile should not clear the flag.
		files.updateFileEntry("mods/x.pw.toml", "sha256", "def", false)

		if !files["mods/x.pw.toml"].IsMetaFile() {
			t.Error("IsMetaFile flag was cleared by a non-metafile update")
		}
	})
}

func TestIndexFilesTomlRoundTrip(t *testing.T) {
	t.Run("single file round-trips", func(t *testing.T) {
		rep := indexFilesTomlRepresentation{
			{File: "mods/sodium.pw.toml", Hash: "abc", HashFormat: "sha256", MetaFile: true},
		}

		mem := rep.toMemoryRep()
		back := mem.toTomlRep()

		if !reflect.DeepEqual([]indexFile(rep), []indexFile(back)) {
			t.Errorf("round-trip mismatch:\n  before: %+v\n   after: %+v", rep, back)
		}
	})

	t.Run("multiple-alias entries are coalesced and re-expanded", func(t *testing.T) {
		rep := indexFilesTomlRepresentation{
			{File: "mods/x.jar", Alias: "", Hash: "a", HashFormat: "sha256"},
			{File: "mods/x.jar", Alias: "alt/x.jar", Hash: "b", HashFormat: "sha256"},
		}

		mem := rep.toMemoryRep()

		holder, ok := mem["mods/x.jar"]
		if !ok {
			t.Fatal("mods/x.jar missing from memory rep")
		}

		if _, ok := holder.(*indexFileMultipleAlias); !ok {
			t.Errorf("expected *indexFileMultipleAlias holder, got %T", holder)
		}

		back := mem.toTomlRep()
		if len(back) != 2 {
			t.Fatalf("toTomlRep returned %d entries, want 2", len(back))
		}
	})

	t.Run("toTomlRep sorts by file then alias", func(t *testing.T) {
		rep := indexFilesTomlRepresentation{
			{File: "z.jar", Hash: "1", HashFormat: "sha256"},
			{File: "a.jar", Alias: "b", Hash: "2", HashFormat: "sha256"},
			{File: "a.jar", Alias: "a", Hash: "3", HashFormat: "sha256"},
		}

		mem := rep.toMemoryRep()
		out := mem.toTomlRep()

		if out[0].File != "a.jar" || out[0].Alias != "a" {
			t.Errorf("first entry: got %+v", out[0])
		}

		if out[1].File != "a.jar" || out[1].Alias != "b" {
			t.Errorf("second entry: got %+v", out[1])
		}

		if out[2].File != "z.jar" {
			t.Errorf("third entry: got %+v", out[2])
		}
	})
}
