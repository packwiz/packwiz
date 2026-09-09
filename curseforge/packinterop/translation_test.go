package packinterop

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/packwiz/packwiz/core"
)

func TestWriteManifestFromPack_RoundTrip(t *testing.T) {
	// Write a pack out to a manifest, parse it back into
	// cursePackMeta, and verify all the round-tripped fields.
	pack := core.Pack{
		Name:    "TestPack",
		Version: "1.0.0",
		Author:  "tester",
		Versions: map[string]string{
			"minecraft": "1.20.1",
			"fabric":    "0.15.0",
		},
	}

	refs := []AddonFileReference{
		{ProjectID: 100, FileID: 1000, OptionalDisabled: false},
		{ProjectID: 200, FileID: 2000, OptionalDisabled: true},
	}

	var buf bytes.Buffer
	if err := WriteManifestFromPack(pack, refs, 42, &buf); err != nil {
		t.Fatalf("WriteManifestFromPack: %v", err)
	}

	// Parse the manifest back. We use a local struct mirror to read
	// the on-disk shape directly instead of round-tripping through
	// cursePackMeta (which unexports importSrc and has json-tagged
	// nested struct fields that don't deserialize cleanly without
	// the unexported importSrc dependency).
	var m cursePackMeta
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if m.NameInternal != "TestPack" {
		t.Errorf("name = %q, want TestPack", m.NameInternal)
	}

	if m.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", m.Version)
	}

	if m.Author != "tester" {
		t.Errorf("author = %q, want tester", m.Author)
	}

	if m.ProjectID != 42 {
		t.Errorf("projectID = %d, want 42", m.ProjectID)
	}

	if m.ManifestType != "minecraftModpack" {
		t.Errorf("manifestType = %q, want minecraftModpack", m.ManifestType)
	}

	if m.Overrides != "overrides" {
		t.Errorf("overrides = %q, want overrides", m.Overrides)
	}

	if m.Minecraft.Version != "1.20.1" {
		t.Errorf("minecraft.version = %q, want 1.20.1", m.Minecraft.Version)
	}

	if len(m.Minecraft.ModLoaders) != 1 {
		t.Fatalf("modloader count = %d, want 1", len(m.Minecraft.ModLoaders))
	}

	if m.Minecraft.ModLoaders[0].ID != "fabric-0.15.0" {
		t.Errorf("modloader[0].ID = %q, want fabric-0.15.0", m.Minecraft.ModLoaders[0].ID)
	}

	if !m.Minecraft.ModLoaders[0].Primary {
		t.Error("modloader[0].Primary = false, want true")
	}

	if len(m.Files) != 2 {
		t.Fatalf("files count = %d, want 2", len(m.Files))
	}

	// OptionalDisabled is inverted into Required on write.
	if m.Files[0].ProjectID != 100 || m.Files[0].FileID != 1000 || !m.Files[0].Required {
		t.Errorf("file[0] = %+v, want ProjectID=100 FileID=1000 Required=true", m.Files[0])
	}

	if m.Files[1].ProjectID != 200 || m.Files[1].FileID != 2000 || m.Files[1].Required {
		t.Errorf("file[1] = %+v, want ProjectID=200 FileID=2000 Required=false", m.Files[1])
	}
}

func TestWriteManifestFromPack_LoaderPriority(t *testing.T) {
	// When multiple loaders are present in pack.Versions, the function
	// picks the first match in priority order: fabric → forge →
	// neoforge → quilt. The if/else-if chain stops at the first match,
	// so fabric beats everything else when both are set.
	cases := []struct {
		name     string
		versions map[string]string
		wantID   string
	}{
		{
			name:     "fabric only",
			versions: map[string]string{"minecraft": "1.20.1", "fabric": "0.15.0"},
			wantID:   "fabric-0.15.0",
		},
		{
			name:     "forge only",
			versions: map[string]string{"minecraft": "1.20.1", "forge": "47.1.0"},
			wantID:   "forge-47.1.0",
		},
		{
			name:     "neoforge only",
			versions: map[string]string{"minecraft": "1.20.1", "neoforge": "20.4.230"},
			wantID:   "neoforge-20.4.230",
		},
		{
			name:     "quilt only",
			versions: map[string]string{"minecraft": "1.20.1", "quilt": "0.21.0"},
			wantID:   "quilt-0.21.0",
		},
		{
			// fabric+forge → fabric wins per the if-chain order.
			name:     "fabric beats forge when both present",
			versions: map[string]string{"minecraft": "1.20.1", "fabric": "0.15.0", "forge": "47.1.0"},
			wantID:   "fabric-0.15.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteManifestFromPack(core.Pack{Versions: tc.versions}, nil, 0, &buf)
			if err != nil {
				t.Fatalf("WriteManifestFromPack: %v", err)
			}

			var m cursePackMeta
			if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if len(m.Minecraft.ModLoaders) != 1 {
				t.Fatalf("expected 1 modloader, got %d", len(m.Minecraft.ModLoaders))
			}

			if m.Minecraft.ModLoaders[0].ID != tc.wantID {
				t.Errorf("modloader ID = %q, want %q", m.Minecraft.ModLoaders[0].ID, tc.wantID)
			}
		})
	}
}
