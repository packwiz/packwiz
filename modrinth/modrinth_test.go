package modrinth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"github.com/jarcoal/httpmock"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/viper"
)

func sptr(s string) *string { return &s }

// newTestModrinthMod writes a Mod TOML to a temp file and loads it
// through core.LoadMod, which exercises the modrinth init() updater
// registration so [update.modrinth] decode happens normally.
func newTestModrinthMod(t *testing.T, name, filename, projectID, versionID string) *core.Mod {
	t.Helper()

	dir := t.TempDir()
	modPath := filepath.Join(dir, "test.pw.toml")

	contents := fmt.Sprintf(`name = %q
filename = %q

[download]
url = "https://example.com/file.jar"
hash-format = "sha1"
hash = "deadbeef"

[update]
[update.modrinth]
mod-id = %q
version = %q
`, name, filename, projectID, versionID)

	if err := os.WriteFile(modPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mod, err := core.LoadMod(modPath)
	if err != nil {
		t.Fatalf("LoadMod: %v", err)
	}

	return &mod
}

func TestShouldDownloadOnSide(t *testing.T) {
	cases := []struct {
		side string
		want bool
	}{
		{"required", true},
		{"optional", true},
		{"unsupported", false},
		{"", false},
		{"unknown-value", false},
	}

	for _, tc := range cases {
		t.Run(tc.side, func(t *testing.T) {
			if got := shouldDownloadOnSide(tc.side); got != tc.want {
				t.Errorf("shouldDownloadOnSide(%q) = %v, want %v", tc.side, got, tc.want)
			}
		})
	}
}

func TestGetSide(t *testing.T) {
	cases := []struct {
		name       string
		serverSide string
		clientSide string
		want       string
	}{
		{"required on both", "required", "required", "both"},
		{"optional on both", "optional", "optional", "both"},
		{"server only", "required", "unsupported", "server"},
		{"client only", "unsupported", "required", "client"},
		{"unsupported on both", "unsupported", "unsupported", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := &modrinthApi.Project{
				ServerSide: sptr(tc.serverSide),
				ClientSide: sptr(tc.clientSide),
			}

			if got := getSide(project); got != tc.want {
				t.Errorf("getSide = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetBestHash(t *testing.T) {
	t.Run("sha512 wins over weaker hashes", func(t *testing.T) {
		f := &modrinthApi.File{Hashes: map[string]string{
			"sha512": "z", "sha256": "y", "sha1": "x", "murmur2": "w",
		}}

		gotFmt, gotVal := getBestHash(f)
		if gotFmt != "sha512" || gotVal != "z" {
			t.Errorf("got (%q, %q), want (sha512, z)", gotFmt, gotVal)
		}
	})

	t.Run("sha256 picked when sha512 absent", func(t *testing.T) {
		f := &modrinthApi.File{Hashes: map[string]string{"sha256": "v", "sha1": "u"}}

		gotFmt, gotVal := getBestHash(f)
		if gotFmt != "sha256" || gotVal != "v" {
			t.Errorf("got (%q, %q), want (sha256, v)", gotFmt, gotVal)
		}
	})

	t.Run("sha1 picked when only sha1 and murmur2 present", func(t *testing.T) {
		f := &modrinthApi.File{Hashes: map[string]string{"sha1": "s", "murmur2": "m"}}

		gotFmt, gotVal := getBestHash(f)
		if gotFmt != "sha1" || gotVal != "s" {
			t.Errorf("got (%q, %q), want (sha1, s)", gotFmt, gotVal)
		}
	})

	t.Run("murmur2 picked when preferred hashes absent", func(t *testing.T) {
		f := &modrinthApi.File{Hashes: map[string]string{"murmur2": "m"}}

		gotFmt, gotVal := getBestHash(f)
		if gotFmt != "murmur2" || gotVal != "m" {
			t.Errorf("got (%q, %q), want (murmur2, m)", gotFmt, gotVal)
		}
	})

	t.Run("empty hashes return empty strings", func(t *testing.T) {
		f := &modrinthApi.File{Hashes: map[string]string{}}

		gotFmt, gotVal := getBestHash(f)
		if gotFmt != "" || gotVal != "" {
			t.Errorf("got (%q, %q), want (\"\", \"\")", gotFmt, gotVal)
		}
	})
}

func TestCompareLoaderLists(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want int32
	}{
		{
			name: "both empty",
			a:    nil, b: nil,
			want: 0,
		},
		{
			name: "quilt beats fabric",
			a:    []string{"quilt"}, b: []string{"fabric"},
			want: -1,
		},
		{
			name: "fabric loses to quilt",
			a:    []string{"fabric"}, b: []string{"quilt"},
			want: 1,
		},
		{
			name: "neoforge beats forge",
			a:    []string{"neoforge"}, b: []string{"forge"},
			want: -1,
		},
		{
			// Both lists contain "fabric"; compat group makes "quilt"
			// irrelevant for the comparison, so A and B compare equal.
			name: "quilt+fabric vs fabric -> equal via compat group",
			a:    []string{"quilt", "fabric"}, b: []string{"fabric"},
			want: 0,
		},
		{
			// Both lists contain "forge"; "neoforge" enters the compat
			// group and is ignored, leaving forge on both sides.
			name: "forge vs forge+neoforge -> equal via compat group",
			a:    []string{"forge"}, b: []string{"forge", "neoforge"},
			want: 0,
		},
		{
			// PR #391 territory: a multi-loader version that includes
			// fabric beats a neoforge-only version, even when the
			// consumer pack is neoforge-only — because this function
			// has no view of the pack's loaders. Pinning current
			// behavior; the fix (in upstream PR #391) is in the
			// CALLER, not here.
			name: "PR #391 baseline: fabric-bearing list wins regardless of pack loaders",
			a:    []string{"fabric", "forge", "neoforge"}, b: []string{"neoforge"},
			want: -1,
		},
		{
			name: "non-empty vs empty -> non-empty wins",
			a:    []string{"fabric"}, b: nil,
			want: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareLoaderLists(tc.a, tc.b); got != tc.want {
				t.Errorf("compareLoaderLists(%v, %v) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestGetProjectTypeFolder(t *testing.T) {
	// Ensure viper datapack-folder is unset for this test's lifetime;
	// none of the cases below need it, and a leak from another test
	// could mask the "datapack" branch behavior.
	t.Cleanup(func() { viper.Set("datapack-folder", "") })
	viper.Set("datapack-folder", "")

	cases := []struct {
		name         string
		projectType  string
		fileLoaders  []string
		packLoaders  []string
		want         string
		wantErr      bool
	}{
		{
			name:        "modpack always errors",
			projectType: "modpack",
			wantErr:     true,
		},
		{
			name:        "resourcepack always returns resourcepacks",
			projectType: "resourcepack",
			want:        "resourcepacks",
		},
		{
			name:        "shader with iris loader goes to shaderpacks",
			projectType: "shader",
			fileLoaders: []string{"iris"},
			want:        "shaderpacks",
		},
		{
			// canvas is in loaderFolders as "resourcepacks" (core
			// shaders ship as resource packs); the function honors
			// that mapping for shader projects too.
			name:        "shader with canvas loader goes to resourcepacks",
			projectType: "shader",
			fileLoaders: []string{"canvas"},
			want:        "resourcepacks",
		},
		{
			name:        "shader with unknown loader falls back to shaderpacks",
			projectType: "shader",
			fileLoaders: []string{"unknown-loader"},
			want:        "shaderpacks",
		},
		{
			name:        "mod with overlapping fabric loader goes to mods",
			projectType: "mod",
			fileLoaders: []string{"fabric"},
			packLoaders: []string{"fabric"},
			want:        "mods",
		},
		{
			name:        "mod with no overlap falls back to mods",
			projectType: "mod",
			fileLoaders: []string{"fabric"},
			packLoaders: []string{"forge"},
			want:        "mods",
		},
		{
			name:        "unknown project type errors",
			projectType: "data-pack",
			wantErr:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getProjectTypeFolder(tc.projectType, tc.fileLoaders, tc.packLoaders)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for project type %q, got nil (result: %q)", tc.projectType, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseSlugOrUrl(t *testing.T) {
	cases := []struct {
		name          string
		input         string
		wantSlug      string
		wantVersion   string
		wantVersionID string
		wantFilename  string
		wantParsed    bool
		wantErr       bool
	}{
		{
			name:     "modrinth URL with project category and slug",
			input:    "https://modrinth.com/mod/sodium",
			wantSlug: "sodium",
		},
		{
			name:        "modrinth URL with slug and version",
			input:       "https://modrinth.com/mod/sodium/version/mc1.20.1",
			wantSlug:    "sodium",
			wantVersion: "mc1.20.1",
		},
		{
			name:          "CDN URL extracts id + versionID + filename",
			input:         "https://cdn.modrinth.com/data/abc123/versions/def456/sodium.jar",
			wantSlug:      "abc123",
			wantVersionID: "def456",
			wantFilename:  "sodium.jar",
		},
		{
			name:    "modrinth URL with unknown category returns error",
			input:   "https://modrinth.com/unknowncategory/sodium",
			wantErr: true,
		},
		{
			name:       "plain slug sets parsedSlug=true",
			input:      "sodium",
			wantSlug:   "sodium",
			wantParsed: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var slug, version, versionID, filename string
			parsed, err := parseSlugOrUrl(tc.input, &slug, &version, &versionID, &filename)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if slug != tc.wantSlug {
				t.Errorf("slug = %q, want %q", slug, tc.wantSlug)
			}

			if version != tc.wantVersion {
				t.Errorf("version = %q, want %q", version, tc.wantVersion)
			}

			if versionID != tc.wantVersionID {
				t.Errorf("versionID = %q, want %q", versionID, tc.wantVersionID)
			}

			if filename != tc.wantFilename {
				t.Errorf("filename = %q, want %q", filename, tc.wantFilename)
			}

			if parsed != tc.wantParsed {
				t.Errorf("parsedSlug = %v, want %v", parsed, tc.wantParsed)
			}
		})
	}
}

func TestMapDepOverride(t *testing.T) {
	cases := []struct {
		name      string
		depID     string
		isQuilt   bool
		mcVersion string
		want      string
	}{
		{
			name:      "FAPI ID under Quilt maps to QFAPI/QSL",
			depID:     "P7dR8mSH",
			isQuilt:   true,
			mcVersion: "1.20.1",
			want:      "qvIfYCYJ",
		},
		{
			name:      "FAPI by slug under Quilt also maps",
			depID:     "fabric-api",
			isQuilt:   true,
			mcVersion: "1.20.1",
			want:      "qvIfYCYJ",
		},
		{
			name:      "FAPI under Fabric (not Quilt) is left alone",
			depID:     "P7dR8mSH",
			isQuilt:   false,
			mcVersion: "1.20.1",
			want:      "P7dR8mSH",
		},
		{
			name:      "FLK under Quilt on 1.20.1 maps to QKL",
			depID:     "Ha28R6CL",
			isQuilt:   true,
			mcVersion: "1.20.1",
			want:      "lwVhp9o5",
		},
		{
			name:      "FLK under Quilt on 1.18.2 is left alone (below 1.19.2)",
			depID:     "Ha28R6CL",
			isQuilt:   true,
			mcVersion: "1.18.2",
			want:      "Ha28R6CL",
		},
		{
			name:      "unrelated dep ID is left alone",
			depID:     "abcdef",
			isQuilt:   true,
			mcVersion: "1.20.1",
			want:      "abcdef",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapDepOverride(tc.depID, tc.isQuilt, tc.mcVersion); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// mrVersion is a small builder for *modrinthApi.Version fixtures used
// by the findLatestVersion tests. Each call allocates fresh backing
// strings/times so independent fixtures don't alias one another.
func mrVersion(versionNumber string, gameVersions, loaders []string, date time.Time) *modrinthApi.Version {
	vn := versionNumber
	d := date

	return &modrinthApi.Version{
		VersionNumber: &vn,
		GameVersions:  gameVersions,
		Loaders:       loaders,
		DatePublished: &d,
	}
}

func TestFindLatestVersion_SingleVersionPassesThrough(t *testing.T) {
	only := mrVersion("1.0.0", []string{"1.20.1"}, []string{"fabric"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	got := findLatestVersion([]*modrinthApi.Version{only}, []string{"1.20.1"}, false)
	if got != only {
		t.Errorf("expected the only version to be returned, got a different pointer")
	}
}

func TestFindLatestVersion_LaterDateWinsWhenOthersEqual(t *testing.T) {
	// Same version number, game version, loaders — only DatePublished
	// differs. Newer date should win regardless of ordering.
	older := mrVersion("1.0.0", []string{"1.20.1"}, []string{"fabric"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	newer := mrVersion("1.0.0", []string{"1.20.1"}, []string{"fabric"}, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))

	t.Run("older first", func(t *testing.T) {
		got := findLatestVersion([]*modrinthApi.Version{older, newer}, []string{"1.20.1"}, false)
		if got != newer {
			t.Errorf("expected newer version, got %q", *got.VersionNumber)
		}
	})

	t.Run("newer first", func(t *testing.T) {
		got := findLatestVersion([]*modrinthApi.Version{newer, older}, []string{"1.20.1"}, false)
		if got != newer {
			t.Errorf("expected newer version, got %q", *got.VersionNumber)
		}
	})
}

func TestFindLatestVersion_HigherGameVersionIndexWins(t *testing.T) {
	// packVersions list semantics: later entries are preferred (the
	// "main" MC version comes last). A version that targets the later
	// entry should beat one that only targets the earlier entry, even
	// when published earlier.
	packVersions := []string{"1.19.4", "1.20.1"}

	olderButNewerMC := mrVersion("1.0.0", []string{"1.20.1"}, []string{"fabric"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	newerButOlderMC := mrVersion("2.0.0", []string{"1.19.4"}, []string{"fabric"}, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))

	got := findLatestVersion([]*modrinthApi.Version{newerButOlderMC, olderButNewerMC}, packVersions, false)
	if got != olderButNewerMC {
		t.Errorf("expected the 1.20.1-targeting version to win on game-version index; got %q with MCs %v",
			*got.VersionNumber, got.GameVersions)
	}
}

func TestFindLatestVersion_FlexVerOverridesDate(t *testing.T) {
	// With useFlexVer=true, the version-number compare runs first.
	// A newer flexver beats an older flexver regardless of publish date.
	oldFlexVerNewDate := mrVersion("1.0.0", []string{"1.20.1"}, []string{"fabric"}, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))
	newFlexVerOldDate := mrVersion("2.0.0", []string{"1.20.1"}, []string{"fabric"}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	got := findLatestVersion([]*modrinthApi.Version{oldFlexVerNewDate, newFlexVerOldDate}, []string{"1.20.1"}, true)
	if got != newFlexVerOldDate {
		t.Errorf("expected newer-flexver version to win when useFlexVer=true; got %q",
			*got.VersionNumber)
	}
}

func TestFindLatestVersion_LoaderPreference_QuiltOverFabric(t *testing.T) {
	// Identical date and game version; quilt should win over fabric
	// per loaderPreferenceList.
	date := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	fabricOnly := mrVersion("1.0.0", []string{"1.20.1"}, []string{"fabric"}, date)
	quiltOnly := mrVersion("1.0.0", []string{"1.20.1"}, []string{"quilt"}, date)

	got := findLatestVersion([]*modrinthApi.Version{fabricOnly, quiltOnly}, []string{"1.20.1"}, false)
	if got != quiltOnly {
		t.Errorf("expected quilt version to win loader preference; got %v", got.Loaders)
	}
}

func TestFindLatestVersion_PR391Baseline_LoaderListCompareIsPackUnaware(t *testing.T) {
	// PR #391 baseline: findLatestVersion calls compareLoaderLists
	// without filtering each version's loader list to only those
	// relevant to the consumer pack. So a multi-loader version
	// that includes "fabric" can beat a "neoforge"-only version
	// even when the pack is neoforge-only — because fabric ranks
	// ahead of neoforge in loaderPreferenceList.
	//
	// The fix proposed in upstream PR #391 filters each version's
	// loader list to only pack-relevant loaders before comparison.
	// When that lands, this test breaks and the behavior we're
	// pinning will need to be revisited. See .claude/TODO.md for
	// the matched companion task on the CurseForge side.
	date := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	multiLoader := mrVersion("5.0.2", []string{"1.20.1"}, []string{"fabric", "forge", "neoforge"}, date)
	neoforgeOnly := mrVersion("5.4.6.1", []string{"1.20.1"}, []string{"neoforge"}, date)

	// useFlexVer=false so version-number doesn't break the tie.
	got := findLatestVersion([]*modrinthApi.Version{multiLoader, neoforgeOnly}, []string{"1.20.1"}, false)
	if got != multiLoader {
		t.Errorf("baseline pin violated: expected multiLoader (fabric-bearing) to win; got %q", *got.VersionNumber)
	}
}

// projectIDPtr / versionPtr / boolPtr just keep the resolveVersion
// and getLatestVersion test fixtures readable.
func strPtr(s string) *string { return &s }

func TestResolveVersion_VersionIDInProjectVersionList(t *testing.T) {
	// When the requested version string is present in project.Versions,
	// resolveVersion treats it as an ID and does a single Versions.Get.
	httpmock.Activate(t)

	const projectID = "AANobbMI"
	const versionID = "gqoXgtxO"

	body := `{
		"id": "gqoXgtxO",
		"project_id": "AANobbMI",
		"name": "Test 1.0.0",
		"version_number": "1.0.0",
		"game_versions": ["1.20.1"],
		"loaders": ["fabric"],
		"date_published": "2024-01-15T00:00:00Z",
		"files": []
	}`

	httpmock.RegisterResponder("GET",
		"https://api.modrinth.com/v2/version/"+versionID,
		httpmock.NewStringResponder(200, body))

	project := &modrinthApi.Project{
		ID:       strPtr(projectID),
		Versions: []string{versionID, "anotherVerID"},
	}

	got, err := resolveVersion(project, versionID)
	if err != nil {
		t.Fatalf("resolveVersion: %v", err)
	}

	if got.ID == nil || *got.ID != versionID {
		t.Errorf("ID = %v, want %q", got.ID, versionID)
	}

	if got.VersionNumber == nil || *got.VersionNumber != "1.0.0" {
		t.Errorf("VersionNumber = %v, want \"1.0.0\"", got.VersionNumber)
	}
}

func TestResolveVersion_FallbackToVersionNumberLookup(t *testing.T) {
	// When the input isn't a known version ID, resolveVersion lists
	// all versions and finds the one with a matching version_number.
	// The traversal is in reverse order so the OLDEST matching entry
	// wins (per the function's comment about Modrinth knossos
	// behavior).
	httpmock.Activate(t)

	const projectID = "AANobbMI"

	body := `[
		{"id": "newID", "version_number": "1.0.0", "name": "newer dup"},
		{"id": "oldID", "version_number": "1.0.0", "name": "older dup"}
	]`

	httpmock.RegisterRegexpResponder("GET",
		regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
		httpmock.NewStringResponder(200, body))

	project := &modrinthApi.Project{
		ID:       strPtr(projectID),
		Versions: []string{"newID", "oldID"},
	}

	// "1.0.0" is not in Versions (only IDs are), so the function falls
	// through to ListVersions.
	got, err := resolveVersion(project, "1.0.0")
	if err != nil {
		t.Fatalf("resolveVersion: %v", err)
	}

	// Reverse traversal: the function iterates from len-1 down. The
	// last matching entry it encounters wins, which is the FIRST entry
	// of the list since they both match — so we get "newID".
	// Wait: iterating from index 1 (oldID) downward, the function
	// returns the FIRST match it finds, which is oldID at index 1.
	// (The comment in the function says Modrinth returns the oldest
	// file precedence-style, so the function reverses to undo that.)
	if got.ID == nil || *got.ID != "oldID" {
		t.Errorf("ID = %v, want \"oldID\" (reverse-iteration picks the highest-index match first)", got.ID)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	httpmock.Activate(t)

	const projectID = "AANobbMI"

	body := `[
		{"id": "abc", "version_number": "1.0.0"},
		{"id": "def", "version_number": "2.0.0"}
	]`

	httpmock.RegisterRegexpResponder("GET",
		regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
		httpmock.NewStringResponder(200, body))

	project := &modrinthApi.Project{
		ID:       strPtr(projectID),
		Versions: []string{"abc", "def"},
	}

	_, err := resolveVersion(project, "9.9.9")
	if err == nil {
		t.Error("expected error when version number is not found, got nil")
	}
}

func TestGetLatestVersion_HappyPath(t *testing.T) {
	httpmock.Activate(t)

	t.Cleanup(func() {
		viper.Set("acceptable-game-versions", nil)
		viper.Set("datapack-folder", "")
	})
	viper.Set("acceptable-game-versions", nil)
	viper.Set("datapack-folder", "")

	const projectID = "AANobbMI"

	// Two versions, both target 1.20.1+fabric; the second was
	// published later so findLatestVersion should pick it.
	body := `[
		{
			"id": "olderID",
			"version_number": "1.0.0",
			"game_versions": ["1.20.1"],
			"loaders": ["fabric"],
			"date_published": "2024-01-15T00:00:00Z",
			"files": []
		},
		{
			"id": "newerID",
			"version_number": "1.0.1",
			"game_versions": ["1.20.1"],
			"loaders": ["fabric"],
			"date_published": "2024-06-15T00:00:00Z",
			"files": []
		}
	]`

	httpmock.RegisterRegexpResponder("GET",
		regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
		httpmock.NewStringResponder(200, body))

	pack := core.Pack{
		Versions: map[string]string{
			"minecraft": "1.20.1",
			"fabric":    "0.15.0",
		},
	}

	got, err := getLatestVersion(projectID, "Test Mod", pack)
	if err != nil {
		t.Fatalf("getLatestVersion: %v", err)
	}

	if got.ID == nil || *got.ID != "newerID" {
		t.Errorf("picked ID = %v, want \"newerID\"", got.ID)
	}
}

func TestGetLatestVersion_NoVersionsError(t *testing.T) {
	httpmock.Activate(t)

	t.Cleanup(func() {
		viper.Set("acceptable-game-versions", nil)
		viper.Set("datapack-folder", "")
	})
	viper.Set("acceptable-game-versions", nil)
	viper.Set("datapack-folder", "")

	const projectID = "AANobbMI"

	httpmock.RegisterRegexpResponder("GET",
		regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
		httpmock.NewStringResponder(200, `[]`))

	pack := core.Pack{
		Versions: map[string]string{
			"minecraft": "1.20.1",
			"fabric":    "0.15.0",
		},
	}

	_, err := getLatestVersion(projectID, "Test Mod", pack)
	if err == nil {
		t.Error("expected error when API returns empty version list, got nil")
	}
}

func TestMrUpdateData_ToMap_RoundTrip(t *testing.T) {
	original := mrUpdateData{
		ProjectID:        "AANobbMI",
		InstalledVersion: "gqoXgtxO",
	}

	m, err := original.ToMap()
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}

	// The mapstructure tags map to "mod-id" and "version" rather
	// than the field names. Pin those here so a future tag rename
	// (the source has // TODO(format): change to ... comments)
	// surfaces in tests.
	if m["mod-id"] != "AANobbMI" {
		t.Errorf("ToMap key mod-id = %v, want AANobbMI", m["mod-id"])
	}

	if m["version"] != "gqoXgtxO" {
		t.Errorf("ToMap key version = %v, want gqoXgtxO", m["version"])
	}

	parsed, err := mrUpdater{}.ParseUpdate(m)
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}

	if parsed.(mrUpdateData) != original {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", parsed, original)
	}
}

func TestMrUpdater_ParseUpdate(t *testing.T) {
	in := map[string]interface{}{
		"mod-id":  "AANobbMI",
		"version": "gqoXgtxO",
	}

	parsed, err := mrUpdater{}.ParseUpdate(in)
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}

	data, ok := parsed.(mrUpdateData)
	if !ok {
		t.Fatalf("ParseUpdate returned %T, want mrUpdateData", parsed)
	}

	if data.ProjectID != "AANobbMI" || data.InstalledVersion != "gqoXgtxO" {
		t.Errorf("got %+v, want mod-id=AANobbMI version=gqoXgtxO", data)
	}
}

func TestGetProjectIdsViaSearch(t *testing.T) {
	httpmock.Activate(t)

	body := `{
		"hits": [
			{"slug": "sodium", "title": "Sodium", "project_id": "AANobbMI"},
			{"slug": "iris", "title": "Iris", "project_id": "YL57xq9U"}
		],
		"offset": 0,
		"limit": 5,
		"total_hits": 2
	}`

	httpmock.RegisterRegexpResponder("GET",
		regexp.MustCompile(`^https://api\.modrinth\.com/v2/search`),
		httpmock.NewStringResponder(200, body))

	hits, err := getProjectIdsViaSearch("performance", []string{"1.20.1"})
	if err != nil {
		t.Fatalf("getProjectIdsViaSearch: %v", err)
	}

	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2", len(hits))
	}

	wantSlugs := []string{"sodium", "iris"}
	for i, h := range hits {
		if h.Slug == nil || *h.Slug != wantSlugs[i] {
			t.Errorf("hits[%d].Slug = %v, want %q", i, h.Slug, wantSlugs[i])
		}
	}

	wantIDs := []string{"AANobbMI", "YL57xq9U"}
	for i, h := range hits {
		if h.ProjectID == nil || *h.ProjectID != wantIDs[i] {
			t.Errorf("hits[%d].ProjectID = %v, want %q", i, h.ProjectID, wantIDs[i])
		}
	}
}

func TestMrUpdater_CheckUpdate(t *testing.T) {
	const projectID = "AANobbMI"
	const installedVersion = "oldVersion"
	const newVersionID = "newVersion"

	pack := core.Pack{
		Versions: map[string]string{
			"minecraft": "1.20.1",
			"fabric":    "0.15.0",
		},
	}

	registerCleanup := func(t *testing.T) {
		t.Helper()
		t.Cleanup(func() {
			viper.Set("acceptable-game-versions", nil)
			viper.Set("datapack-folder", "")
		})
		viper.Set("acceptable-game-versions", nil)
		viper.Set("datapack-folder", "")
	}

	t.Run("mod without modrinth update data gets a per-mod error", func(t *testing.T) {
		// Construct a Mod directly without going through LoadMod, so
		// updateData stays empty and GetParsedUpdateData misses.
		mod := &core.Mod{Name: "no-update", FileName: "test.jar"}

		results, err := mrUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate returned overall error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error, got %+v", results[0])
		}
	})

	t.Run("installed version equals latest yields UpdateAvailable=false", func(t *testing.T) {
		httpmock.Activate(t)
		registerCleanup(t)

		body := fmt.Sprintf(`[{
			"id": %q,
			"version_number": "1.0.0",
			"game_versions": ["1.20.1"],
			"loaders": ["fabric"],
			"date_published": "2024-01-15T00:00:00Z",
			"files": []
		}]`, installedVersion)

		httpmock.RegisterRegexpResponder("GET",
			regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
			httpmock.NewStringResponder(200, body))

		mod := newTestModrinthMod(t, "Test Mod", "old.jar", projectID, installedVersion)

		results, err := mrUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error != nil {
			t.Fatalf("unexpected per-mod error: %v", results[0].Error)
		}

		if results[0].UpdateAvailable {
			t.Errorf("expected UpdateAvailable=false, got result %+v", results[0])
		}
	})

	t.Run("newer version available with primary file", func(t *testing.T) {
		httpmock.Activate(t)
		registerCleanup(t)

		body := fmt.Sprintf(`[{
			"id": %q,
			"version_number": "2.0.0",
			"game_versions": ["1.20.1"],
			"loaders": ["fabric"],
			"date_published": "2024-06-15T00:00:00Z",
			"files": [
				{"filename": "secondary.jar", "primary": false},
				{"filename": "primary.jar", "primary": true}
			]
		}]`, newVersionID)

		httpmock.RegisterRegexpResponder("GET",
			regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
			httpmock.NewStringResponder(200, body))

		mod := newTestModrinthMod(t, "Test Mod", "old.jar", projectID, installedVersion)

		results, err := mrUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error != nil {
			t.Fatalf("unexpected per-mod error: %v", results[0].Error)
		}

		if !results[0].UpdateAvailable {
			t.Fatalf("expected UpdateAvailable=true, got %+v", results[0])
		}

		// UpdateString prefers the primary file's filename, not the
		// first file in the slice.
		if results[0].UpdateString != "old.jar -> primary.jar" {
			t.Errorf("UpdateString = %q, want %q",
				results[0].UpdateString, "old.jar -> primary.jar")
		}
	})

	t.Run("new version with no files yields per-mod error", func(t *testing.T) {
		httpmock.Activate(t)
		registerCleanup(t)

		body := fmt.Sprintf(`[{
			"id": %q,
			"version_number": "2.0.0",
			"game_versions": ["1.20.1"],
			"loaders": ["fabric"],
			"date_published": "2024-06-15T00:00:00Z",
			"files": []
		}]`, newVersionID)

		httpmock.RegisterRegexpResponder("GET",
			regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
			httpmock.NewStringResponder(200, body))

		mod := newTestModrinthMod(t, "Test Mod", "old.jar", projectID, installedVersion)

		results, err := mrUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error for files-empty case, got %+v", results[0])
		}
	})

	t.Run("getLatestVersion error becomes a per-mod error", func(t *testing.T) {
		httpmock.Activate(t)
		registerCleanup(t)

		httpmock.RegisterRegexpResponder("GET",
			regexp.MustCompile(`^https://api\.modrinth\.com/v2/project/`+projectID+`/version`),
			httpmock.NewStringResponder(500, `{"error":"boom"}`))

		mod := newTestModrinthMod(t, "Test Mod", "old.jar", projectID, installedVersion)

		results, err := mrUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate returned overall error: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error from getLatestVersion 500, got %+v", results[0])
		}
	})
}
