package curseforge

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/viper"
)

// newTestCurseforgeMod writes a Mod TOML referencing the curseforge
// update plugin, then loads it through core.LoadMod so the registered
// cfUpdater parses the update map.
func newTestCurseforgeMod(t *testing.T, name, filename string, projectID, fileID uint32) *core.Mod {
	t.Helper()

	dir := t.TempDir()
	modPath := filepath.Join(dir, "test.pw.toml")

	contents := fmt.Sprintf(`name = %q
filename = %q

[download]
url = "https://example.com/file.jar"
hash-format = "sha1"
hash = "deadbeef"
mode = "metadata:curseforge"

[update]
[update.curseforge]
project-id = %d
file-id = %d
`, name, filename, projectID, fileID)

	if err := os.WriteFile(modPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mod, err := core.LoadMod(modPath)
	if err != nil {
		t.Fatalf("LoadMod: %v", err)
	}

	return &mod
}

func TestGetCurseforgeVersion(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain release passes through unchanged", "1.20.1", "1.20.1"},
		{"pre-release suffix triggers -Snapshot", "1.18.1-pre1", "1.18.1-Snapshot"},
		{"rc suffix triggers -Snapshot", "1.20-rc1", "1.20-Snapshot"},
		{"22w24a maps to 1.19-Snapshot", "22w24a", "1.19-Snapshot"},
		{"20w20a maps to 1.16-Snapshot per the snapshot table", "20w20a", "1.16-Snapshot"},
		{"non-matching gibberish returns input unchanged", "definitely-not-a-version", "definitely-not-a-version"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getCurseforgeVersion(tc.input); got != tc.want {
				t.Errorf("getCurseforgeVersion(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestGetCurseforgeVersions(t *testing.T) {
	got := getCurseforgeVersions([]string{"1.20.1", "1.18.1-pre1", "1.19.4"})
	want := []string{"1.20.1", "1.18.1-Snapshot", "1.19.4"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got := getCurseforgeVersions(nil); len(got) != 0 {
		t.Errorf("nil input should yield empty slice, got %v", got)
	}
}

func TestParseSlugOrUrl(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		wantGame     string
		wantCategory string
		wantSlug     string
		wantFileID   uint32
	}{
		{
			name:         "curseforge.com URL with category",
			input:        "https://www.curseforge.com/minecraft/mc-mods/sodium",
			wantGame:     "minecraft",
			wantCategory: "mc-mods",
			wantSlug:     "sodium",
		},
		{
			name:         "curseforge.com URL with category and fileID",
			input:        "https://www.curseforge.com/minecraft/mc-mods/sodium/files/4567890",
			wantGame:     "minecraft",
			wantCategory: "mc-mods",
			wantSlug:     "sodium",
			wantFileID:   4567890,
		},
		{
			name:     "legacy minecraft.curseforge.com URL (no category)",
			input:    "https://minecraft.curseforge.com/projects/sodium",
			wantGame: "minecraft",
			wantSlug: "sodium",
		},
		{
			name:     "plain slug",
			input:    "sodium",
			wantSlug: "sodium",
		},
		{
			// Trailing punctuation breaks all three regexes; the function
			// returns zero values with no error.
			name: "non-matching input returns zero values",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			game, category, slug, fileID, err := parseSlugOrUrl(tc.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if game != tc.wantGame {
				t.Errorf("game = %q, want %q", game, tc.wantGame)
			}

			if category != tc.wantCategory {
				t.Errorf("category = %q, want %q", category, tc.wantCategory)
			}

			if slug != tc.wantSlug {
				t.Errorf("slug = %q, want %q", slug, tc.wantSlug)
			}

			if fileID != tc.wantFileID {
				t.Errorf("fileID = %d, want %d", fileID, tc.wantFileID)
			}
		})
	}
}

func TestGetPathForFile(t *testing.T) {
	t.Cleanup(func() {
		viper.Set("meta-folder", "")
		viper.Set("meta-folder-base", "")
	})

	t.Run("known game + class folder mapping", func(t *testing.T) {
		viper.Set("meta-folder", "")
		viper.Set("meta-folder-base", "")
		// gameID 432 = Minecraft, classID 6 = mods.
		got := getPathForFile(432, 6, 0, "sodium")
		want := filepath.Join("mods", "sodium"+core.MetaExtension)

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("unknown class falls back to categoryID mapping", func(t *testing.T) {
		viper.Set("meta-folder", "")
		viper.Set("meta-folder-base", "")
		// classID 999 is unknown, categoryID 5 = plugins.
		got := getPathForFile(432, 999, 5, "luckperms")
		want := filepath.Join("plugins", "luckperms"+core.MetaExtension)

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("unknown game falls back to flat layout", func(t *testing.T) {
		viper.Set("meta-folder", "")
		viper.Set("meta-folder-base", "")
		got := getPathForFile(0, 0, 0, "sodium")
		want := filepath.Join(".", "sodium"+core.MetaExtension)

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("explicit meta-folder wins over folder mapping", func(t *testing.T) {
		viper.Set("meta-folder", "custom-folder")
		viper.Set("meta-folder-base", "")
		// classID 6 would normally map to "mods"; explicit setting overrides.
		got := getPathForFile(432, 6, 0, "sodium")
		want := filepath.Join("custom-folder", "sodium"+core.MetaExtension)

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("meta-folder-base is prepended", func(t *testing.T) {
		viper.Set("meta-folder", "")
		viper.Set("meta-folder-base", "packroot")
		got := getPathForFile(432, 6, 0, "sodium")
		want := filepath.Join("packroot", "mods", "sodium"+core.MetaExtension)

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestGetSearchLoaderType(t *testing.T) {
	cases := []struct {
		name string
		vers map[string]string
		want modloaderType
	}{
		{"only fabric → Fabric", map[string]string{"fabric": "1"}, modloaderTypeFabric},
		{"only forge → Forge", map[string]string{"forge": "1"}, modloaderTypeForge},
		{"fabric + quilt → Any (multiple loaders)", map[string]string{"fabric": "1", "quilt": "1"}, modloaderTypeAny},
		{"fabric + forge → Any (mixed)", map[string]string{"fabric": "1", "forge": "1"}, modloaderTypeAny},
		{"only quilt → Any (no special case)", map[string]string{"quilt": "1"}, modloaderTypeAny},
		{"only neoforge → Any (no special case)", map[string]string{"neoforge": "1"}, modloaderTypeAny},
		{"empty → Any", map[string]string{}, modloaderTypeAny},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := core.Pack{Versions: tc.vers}

			if got := getSearchLoaderType(p); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFilterLoaderTypeIndex(t *testing.T) {
	cases := []struct {
		name          string
		packLoaders   []string
		modLoaderType modloaderType
		wantType      modloaderType
		wantOK        bool
	}{
		{
			name:          "empty packLoaders allows any file",
			packLoaders:   nil,
			modLoaderType: modloaderTypeForge,
			wantType:      modloaderTypeAny,
			wantOK:        true,
		},
		{
			name:          "Any always passes through",
			packLoaders:   []string{"fabric"},
			modLoaderType: modloaderTypeAny,
			wantType:      modloaderTypeAny,
			wantOK:        true,
		},
		{
			name:          "supported loader passes through with its type",
			packLoaders:   []string{"fabric"},
			modLoaderType: modloaderTypeFabric,
			wantType:      modloaderTypeFabric,
			wantOK:        true,
		},
		{
			name:          "unsupported loader is filtered out",
			packLoaders:   []string{"forge"},
			modLoaderType: modloaderTypeFabric,
			wantType:      modloaderTypeAny,
			wantOK:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotOK := filterLoaderTypeIndex(tc.packLoaders, tc.modLoaderType)

			if gotType != tc.wantType || gotOK != tc.wantOK {
				t.Errorf("got (%d, %v), want (%d, %v)", gotType, gotOK, tc.wantType, tc.wantOK)
			}
		})
	}
}

func TestFilterFileInfoLoaderIndex(t *testing.T) {
	cases := []struct {
		name        string
		packLoaders []string
		fileVers    []string
		wantType    modloaderType
		wantOK      bool
	}{
		{
			name:        "empty packLoaders allows any file",
			packLoaders: nil,
			fileVers:    []string{"Fabric", "1.20.1"},
			wantType:    modloaderTypeAny,
			wantOK:      true,
		},
		{
			name:        "file with matching loader returns that loader",
			packLoaders: []string{"fabric"},
			fileVers:    []string{"Fabric", "1.20.1"},
			wantType:    modloaderTypeFabric,
			wantOK:      true,
		},
		{
			name:        "file with non-matching loader is rejected",
			packLoaders: []string{"fabric"},
			fileVers:    []string{"Forge", "1.20.1"},
			wantType:    modloaderTypeAny,
			wantOK:      false,
		},
		{
			// Higher modloaderType index wins: Fabric (4) > Forge (1).
			name:        "when multiple loaders match, higher index wins",
			packLoaders: []string{"fabric", "forge"},
			fileVers:    []string{"Fabric", "Forge", "1.20.1"},
			wantType:    modloaderTypeFabric,
			wantOK:      true,
		},
		{
			// NeoForge (6) > Quilt (5) > Fabric (4).
			name:        "neoforge beats quilt and fabric when all match",
			packLoaders: []string{"fabric", "quilt", "neoforge"},
			fileVers:    []string{"Fabric", "Quilt", "NeoForge", "1.20.1"},
			wantType:    modloaderTypeNeoForge,
			wantOK:      true,
		},
		{
			name:        "no game versions of any kind means no match",
			packLoaders: []string{"fabric"},
			fileVers:    []string{"1.20.1"},
			wantType:    modloaderTypeAny,
			wantOK:      false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := modFileInfo{GameVersions: tc.fileVers}

			gotType, gotOK := filterFileInfoLoaderIndex(tc.packLoaders, info)

			if gotType != tc.wantType || gotOK != tc.wantOK {
				t.Errorf("got (%d, %v), want (%d, %v)", gotType, gotOK, tc.wantType, tc.wantOK)
			}
		})
	}
}

func TestMapDepOverride(t *testing.T) {
	const (
		fapiID  uint32 = 306612
		qfapiID uint32 = 634179
		flkID   uint32 = 308769
		qklID   uint32 = 720410
	)

	cases := []struct {
		name      string
		depID     uint32
		isQuilt   bool
		mcVersion string
		want      uint32
	}{
		{"FAPI under Quilt → QFAPI", fapiID, true, "1.20.1", qfapiID},
		{"FAPI under Fabric is left alone", fapiID, false, "1.20.1", fapiID},
		{"FLK under Quilt on 1.20.1 → QKL", flkID, true, "1.20.1", qklID},
		{"FLK under Quilt on 1.19.0 is left alone (below 1.19.2)", flkID, true, "1.19.0", flkID},
		{"FLK under Quilt on 1.18.2 is left alone (below 1.19.2)", flkID, true, "1.18.2", flkID},
		{"unrelated dep ID is left alone", 12345, true, "1.20.1", 12345},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapDepOverride(tc.depID, tc.isQuilt, tc.mcVersion); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseExportData(t *testing.T) {
	t.Run("round-trip via ToMap → parseExportData", func(t *testing.T) {
		original := cfExportData{ProjectID: 12345}

		m, err := original.ToMap()
		if err != nil {
			t.Fatalf("ToMap: %v", err)
		}

		back, err := parseExportData(m)
		if err != nil {
			t.Fatalf("parseExportData: %v", err)
		}

		if back != original {
			t.Errorf("round-trip mismatch: got %+v, want %+v", back, original)
		}
	})

	t.Run("missing project-id yields zero value", func(t *testing.T) {
		got, err := parseExportData(map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.ProjectID != 0 {
			t.Errorf("got %+v, want zero ProjectID", got)
		}
	})
}

func TestCfUpdateData_ToMap_RoundTrip(t *testing.T) {
	original := cfUpdateData{
		ProjectID: 306612,
		FileID:    4567890,
	}

	m, err := original.ToMap()
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}

	// mapstructure tags map to "project-id" and "file-id".
	if got := m["project-id"]; got != uint32(306612) {
		t.Errorf("ToMap key project-id = %v (type %T), want 306612 uint32", got, got)
	}

	if got := m["file-id"]; got != uint32(4567890) {
		t.Errorf("ToMap key file-id = %v (type %T), want 4567890 uint32", got, got)
	}

	parsed, err := cfUpdater{}.ParseUpdate(m)
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}

	if parsed.(cfUpdateData) != original {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", parsed, original)
	}
}

func TestCfUpdater_ParseUpdate(t *testing.T) {
	in := map[string]interface{}{
		"project-id": uint32(306612),
		"file-id":    uint32(4567890),
	}

	parsed, err := cfUpdater{}.ParseUpdate(in)
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}

	data, ok := parsed.(cfUpdateData)
	if !ok {
		t.Fatalf("ParseUpdate returned %T, want cfUpdateData", parsed)
	}

	if data.ProjectID != 306612 || data.FileID != 4567890 {
		t.Errorf("got %+v, want ProjectID=306612 FileID=4567890", data)
	}
}

// cfFile builds a modFileInfo with the minimum fields findLatestFile
// consumes (ID, FileName, GameVersions). Other fields default to
// zero and are not exercised by the function.
func cfFile(id uint32, fileName string, gameVersions []string) modFileInfo {
	return modFileInfo{
		ID:           id,
		FileName:     fileName,
		GameVersions: gameVersions,
	}
}

func TestFindLatestFile_PicksHigherMCVersionIndex(t *testing.T) {
	// mcVersions list semantics: later entries are preferred (the
	// "main" MC version comes last). A file targeting the later
	// entry should beat one targeting only the earlier entry.
	info := modInfo{LatestFiles: []modFileInfo{
		cfFile(1, "old.jar", []string{"Fabric", "1.19.4"}),
		cfFile(2, "new.jar", []string{"Fabric", "1.20.1"}),
	}}

	id, _, name := findLatestFile(info, []string{"1.19.4", "1.20.1"}, []string{"fabric"})

	if id != 2 || name != "new.jar" {
		t.Errorf("got (id=%d, name=%q), want (2, new.jar)", id, name)
	}
}

func TestFindLatestFile_HigherLoaderTypeWins(t *testing.T) {
	// When both files target the same MC version, the higher
	// modloaderType index wins (NeoForge=6 > Fabric=4 > Forge=1).
	info := modInfo{LatestFiles: []modFileInfo{
		cfFile(1, "fabric.jar", []string{"Fabric", "1.20.1"}),
		cfFile(2, "neoforge.jar", []string{"NeoForge", "1.20.1"}),
	}}

	id, _, name := findLatestFile(info, []string{"1.20.1"}, []string{"fabric", "neoforge"})

	if id != 2 || name != "neoforge.jar" {
		t.Errorf("got (id=%d, name=%q), want (2, neoforge.jar)", id, name)
	}
}

func TestFindLatestFile_HigherIDWinsAsTiebreaker(t *testing.T) {
	// Same MC, same loader — higher file ID wins.
	info := modInfo{LatestFiles: []modFileInfo{
		cfFile(100, "first.jar", []string{"Fabric", "1.20.1"}),
		cfFile(500, "second.jar", []string{"Fabric", "1.20.1"}),
	}}

	id, _, _ := findLatestFile(info, []string{"1.20.1"}, []string{"fabric"})

	if id != 500 {
		t.Errorf("got id=%d, want 500", id)
	}
}

func TestFindLatestFile_SkipsFilesWithUnsupportedLoader(t *testing.T) {
	// Forge-only file in a Fabric-only pack is filtered out by
	// filterFileInfoLoaderIndex.
	info := modInfo{LatestFiles: []modFileInfo{
		cfFile(1, "forge.jar", []string{"Forge", "1.20.1"}),
		cfFile(2, "fabric.jar", []string{"Fabric", "1.20.1"}),
	}}

	id, _, _ := findLatestFile(info, []string{"1.20.1"}, []string{"fabric"})

	if id != 2 {
		t.Errorf("got id=%d, want 2 (forge file should have been skipped)", id)
	}
}

func TestFindLatestFile_SkipsFilesWithoutMatchingMCVersion(t *testing.T) {
	info := modInfo{LatestFiles: []modFileInfo{
		cfFile(1, "old-mc.jar", []string{"Fabric", "1.19.4"}),
		cfFile(2, "current.jar", []string{"Fabric", "1.20.1"}),
	}}

	id, _, _ := findLatestFile(info, []string{"1.20.1"}, []string{"fabric"})

	if id != 2 {
		t.Errorf("got id=%d, want 2 (1.19.4-only file should have been skipped)", id)
	}
}

func TestFindLatestFile_NoMatchReturnsZeroValues(t *testing.T) {
	// All files filter out — neither MC version nor loader matches.
	info := modInfo{LatestFiles: []modFileInfo{
		cfFile(1, "forge.jar", []string{"Forge", "1.19.4"}),
	}}

	id, fileInfoData, name := findLatestFile(info, []string{"1.20.1"}, []string{"fabric"})

	if id != 0 || fileInfoData != nil || name != "" {
		t.Errorf("got (id=%d, fileInfo=%v, name=%q), want all zero", id, fileInfoData, name)
	}
}

func TestFindLatestFile_GameVersionLatestFilesContribute(t *testing.T) {
	// When LatestFiles is empty, the GameVersionLatestFiles loop is
	// the only path to a match. fileInfoData stays nil for those
	// entries (the index doesn't carry full modFileInfo).
	info := modInfo{
		GameVersionLatestFiles: []struct {
			GameVersion string        `json:"gameVersion"`
			ID          uint32        `json:"fileId"`
			Name        string        `json:"filename"`
			FileType    fileType      `json:"releaseType"`
			Modloader   modloaderType `json:"modLoader"`
		}{
			{GameVersion: "1.20.1", ID: 42, Name: "via-index.jar", Modloader: modloaderTypeNeoForge},
		},
	}

	id, fileInfoData, name := findLatestFile(info, []string{"1.20.1"}, []string{"neoforge"})

	if id != 42 || name != "via-index.jar" {
		t.Errorf("got (id=%d, name=%q), want (42, via-index.jar)", id, name)
	}

	if fileInfoData != nil {
		t.Errorf("fileInfoData should be nil for GameVersionLatestFiles entries; got %+v", fileInfoData)
	}
}

func TestCfUpdater_CheckUpdate(t *testing.T) {
	pack := core.Pack{
		Versions: map[string]string{
			"minecraft": "1.20.1",
			"fabric":    "0.15.0",
		},
	}

	t.Run("mod without curseforge update data gets a per-mod error", func(t *testing.T) {
		// Need at least one mod that DOES have CF data so the
		// getModInfoMultiple call has something to return; otherwise
		// the function call goes through cleanly with one mod
		// erroring per-slot and the API mock not strictly required.
		httpmock.Activate(t)
		httpmock.RegisterResponder("POST",
			"https://api.curseforge.com/v1/mods",
			httpmock.NewStringResponder(200, `{"data":[]}`))

		bad := &core.Mod{Name: "no-update", FileName: "bad.jar"}

		results, err := cfUpdater{}.CheckUpdate([]*core.Mod{bad}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate returned overall error: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error for missing update data, got %+v", results[0])
		}
	})

	t.Run("same file ID yields UpdateAvailable=false", func(t *testing.T) {
		httpmock.Activate(t)

		// Server returns one modInfo for our requested project ID.
		// The single latest-file entry has the SAME fileID the mod
		// already has installed, so the function reports no update.
		body := `{"data":[{
			"id": 12345,
			"name": "Test",
			"latestFiles": [
				{"id": 4567, "fileName": "test-1.0.0.jar", "gameVersions": ["Fabric", "1.20.1"]}
			],
			"latestFilesIndexes": []
		}]}`

		httpmock.RegisterResponder("POST",
			"https://api.curseforge.com/v1/mods",
			httpmock.NewStringResponder(200, body))

		mod := newTestCurseforgeMod(t, "Test", "test-1.0.0.jar", 12345, 4567)

		results, err := cfUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error != nil {
			t.Fatalf("unexpected per-mod error: %v", results[0].Error)
		}

		if results[0].UpdateAvailable {
			t.Errorf("expected UpdateAvailable=false (same fileID), got %+v", results[0])
		}
	})

	t.Run("different file ID yields UpdateAvailable=true", func(t *testing.T) {
		httpmock.Activate(t)

		// Latest-file entry has fileID 9999, mod has 4567 installed.
		body := `{"data":[{
			"id": 12345,
			"name": "Test",
			"latestFiles": [
				{"id": 9999, "fileName": "test-2.0.0.jar", "gameVersions": ["Fabric", "1.20.1"]}
			],
			"latestFilesIndexes": []
		}]}`

		httpmock.RegisterResponder("POST",
			"https://api.curseforge.com/v1/mods",
			httpmock.NewStringResponder(200, body))

		mod := newTestCurseforgeMod(t, "Test", "test-1.0.0.jar", 12345, 4567)

		results, err := cfUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error != nil {
			t.Fatalf("unexpected per-mod error: %v", results[0].Error)
		}

		if !results[0].UpdateAvailable {
			t.Fatalf("expected UpdateAvailable=true, got %+v", results[0])
		}

		want := "test-1.0.0.jar -> test-2.0.0.jar"
		if results[0].UpdateString != want {
			t.Errorf("UpdateString = %q, want %q", results[0].UpdateString, want)
		}
	})

	t.Run("findLatestFile returns zero yields UpdateAvailable=false", func(t *testing.T) {
		// Pack is fabric-only but the modInfo only carries a forge
		// file. findLatestFile filters out unsupported loaders and
		// returns 0 → the function reports no update available
		// (rather than an error).
		httpmock.Activate(t)

		body := `{"data":[{
			"id": 12345,
			"name": "Test",
			"latestFiles": [
				{"id": 9999, "fileName": "forge-only.jar", "gameVersions": ["Forge", "1.20.1"]}
			],
			"latestFilesIndexes": []
		}]}`

		httpmock.RegisterResponder("POST",
			"https://api.curseforge.com/v1/mods",
			httpmock.NewStringResponder(200, body))

		mod := newTestCurseforgeMod(t, "Test", "test-1.0.0.jar", 12345, 4567)

		results, err := cfUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].UpdateAvailable {
			t.Errorf("expected UpdateAvailable=false when no matching file exists, got %+v", results[0])
		}
	})

	t.Run("getModInfoMultiple error surfaces as overall error", func(t *testing.T) {
		// Unlike per-mod errors, a failure of the bulk modInfo API
		// call bails out the whole function with an error return.
		httpmock.Activate(t)

		httpmock.RegisterResponder("POST",
			"https://api.curseforge.com/v1/mods",
			httpmock.NewStringResponder(500, `{"error":"boom"}`))

		mod := newTestCurseforgeMod(t, "Test", "test.jar", 12345, 4567)

		_, err := cfUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err == nil {
			t.Error("expected overall error from 500 response, got nil")
		}
	})

	t.Run("missing minecraft version surfaces as overall error", func(t *testing.T) {
		// pack.GetSupportedMCVersions errors when "minecraft" is
		// absent. The function returns that error before any HTTP
		// is attempted.
		t.Cleanup(func() { viper.Set("acceptable-game-versions", nil) })
		viper.Set("acceptable-game-versions", nil)

		emptyPack := core.Pack{Versions: map[string]string{}}

		_, err := cfUpdater{}.CheckUpdate([]*core.Mod{{Name: "x"}}, emptyPack)
		if err == nil {
			t.Error("expected error when pack has no minecraft version, got nil")
		}
	})
}
