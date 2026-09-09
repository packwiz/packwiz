package packinterop

import (
	"reflect"
	"testing"
)

func TestCursePackMeta_Accessors(t *testing.T) {
	m := cursePackMeta{
		NameInternal: "Test Pack",
		Version:      "1.0.0",
		Author:       "tester",
	}

	if got := m.Name(); got != "Test Pack" {
		t.Errorf("Name = %q, want %q", got, "Test Pack")
	}

	if got := m.PackVersion(); got != "1.0.0" {
		t.Errorf("PackVersion = %q, want %q", got, "1.0.0")
	}

	if got := m.PackAuthor(); got != "tester" {
		t.Errorf("PackAuthor = %q, want %q", got, "tester")
	}
}

func TestCursePackMeta_Versions(t *testing.T) {
	t.Run("parses minecraft and modloader IDs", func(t *testing.T) {
		m := cursePackMeta{}
		m.Minecraft.Version = "1.20.1"
		m.Minecraft.ModLoaders = []modLoaderDef{
			{ID: "fabric-0.15.0", Primary: true},
		}

		got := m.Versions()
		want := map[string]string{
			"minecraft": "1.20.1",
			"fabric":    "0.15.0",
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("strips minecraft-version prefix from forge", func(t *testing.T) {
		// Forge versions in this manifest format are emitted as
		// "<mcVersion>-<forgeVersion>"; Versions() peels the prefix
		// so callers get the bare forge version.
		m := cursePackMeta{}
		m.Minecraft.Version = "1.20.1"
		m.Minecraft.ModLoaders = []modLoaderDef{
			{ID: "forge-1.20.1-47.1.0", Primary: true},
		}

		got := m.Versions()
		if got["forge"] != "47.1.0" {
			t.Errorf("forge = %q, want %q", got["forge"], "47.1.0")
		}
	})

	t.Run("modloader ID without a dash is silently dropped", func(t *testing.T) {
		// ID values without a "-" don't match SplitN(..., 2) → len 2,
		// so they're skipped. Pinning current behavior.
		m := cursePackMeta{}
		m.Minecraft.Version = "1.20.1"
		m.Minecraft.ModLoaders = []modLoaderDef{
			{ID: "fabric", Primary: true},
		}

		got := m.Versions()
		if _, ok := got["fabric"]; ok {
			t.Errorf("expected fabric to be absent from versions; got %v", got)
		}
	})
}

func TestCursePackMeta_Mods(t *testing.T) {
	m := cursePackMeta{
		Files: []struct {
			ProjectID uint32 `json:"projectID"`
			FileID    uint32 `json:"fileID"`
			Required  bool   `json:"required"`
		}{
			{ProjectID: 1, FileID: 10, Required: true},
			{ProjectID: 2, FileID: 20, Required: false},
		},
	}

	got := m.Mods()

	if len(got) != 2 {
		t.Fatalf("got %d mods, want 2", len(got))
	}

	// Required:true → OptionalDisabled:false (logical inversion).
	if got[0].ProjectID != 1 || got[0].FileID != 10 || got[0].OptionalDisabled {
		t.Errorf("got[0] = %+v, want ProjectID=1 FileID=10 OptionalDisabled=false", got[0])
	}

	if got[1].ProjectID != 2 || got[1].FileID != 20 || !got[1].OptionalDisabled {
		t.Errorf("got[1] = %+v, want ProjectID=2 FileID=20 OptionalDisabled=true", got[1])
	}
}

func TestCursePackOverrideWrapper_Name(t *testing.T) {
	// The wrapper exists solely to override the Name() method while
	// delegating Open() to the wrapped file. Verify the rename works.
	w := cursePackOverrideWrapper{name: "renamed.txt"}
	if got := w.Name(); got != "renamed.txt" {
		t.Errorf("Name = %q, want %q", got, "renamed.txt")
	}
}
