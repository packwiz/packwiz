package github

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/packwiz/packwiz/core"
)

// newTestGithubMod writes a Mod TOML referencing the github update
// plugin, then loads it through core.LoadMod so the registered
// ghUpdater parses the update map.
func newTestGithubMod(t *testing.T, name, filename, slug, tag, branch, regex string) *core.Mod {
	t.Helper()

	dir := t.TempDir()
	modPath := filepath.Join(dir, "test.pw.toml")

	contents := fmt.Sprintf(`name = %q
filename = %q

[download]
url = "https://example.com/file.jar"
hash-format = "sha256"
hash = "deadbeef"

[update]
[update.github]
slug = %q
tag = %q
branch = %q
regex = %q
`, name, filename, slug, tag, branch, regex)

	if err := os.WriteFile(modPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mod, err := core.LoadMod(modPath)
	if err != nil {
		t.Fatalf("LoadMod: %v", err)
	}

	return &mod
}

func TestGithubRegex(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantSlug  string
		wantMatch bool
	}{
		{
			name:      "plain URL",
			input:     "https://github.com/packwiz/packwiz",
			wantSlug:  "packwiz/packwiz",
			wantMatch: true,
		},
		{
			name:      "URL with www prefix",
			input:     "https://www.github.com/packwiz/packwiz",
			wantSlug:  "packwiz/packwiz",
			wantMatch: true,
		},
		{
			name:      "http (not https)",
			input:     "http://github.com/packwiz/packwiz",
			wantSlug:  "packwiz/packwiz",
			wantMatch: true,
		},
		{
			name:      "extra path segments are ignored after the slug",
			input:     "https://github.com/packwiz/packwiz/releases/tag/v1.0",
			wantSlug:  "packwiz/packwiz",
			wantMatch: true,
		},
		{
			name:      "not a github URL",
			input:     "https://example.com/foo/bar",
			wantMatch: false,
		},
		{
			name:      "plain slug (no protocol) does not match",
			input:     "packwiz/packwiz",
			wantMatch: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matches := GithubRegex.FindStringSubmatch(tc.input)

			gotMatch := len(matches) == 2
			if gotMatch != tc.wantMatch {
				t.Fatalf("match = %v, want %v", gotMatch, tc.wantMatch)
			}

			if tc.wantMatch && matches[1] != tc.wantSlug {
				t.Errorf("slug = %q, want %q", matches[1], tc.wantSlug)
			}
		})
	}
}

func TestGetLatestRelease(t *testing.T) {
	const slug = "packwiz/packwiz"
	const releasesURL = "https://api.github.com/repos/" + slug + "/releases"

	t.Run("no branch returns the first release in the array", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"url":"r1", "tag_name":"v2.0.0", "target_commitish":"main", "name":"Two", "created_at":"2024-06-01T00:00:00Z", "assets":[]},
			{"url":"r2", "tag_name":"v1.0.0", "target_commitish":"main", "name":"One", "created_at":"2024-01-01T00:00:00Z", "assets":[]}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		release, err := getLatestRelease(slug, "")
		if err != nil {
			t.Fatalf("getLatestRelease: %v", err)
		}

		if release.TagName != "v2.0.0" {
			t.Errorf("TagName = %q, want v2.0.0", release.TagName)
		}
	})

	t.Run("branch filter returns the matching release", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"tag_name":"v2.0.0-main", "target_commitish":"main"},
			{"tag_name":"v1.5.0-stable", "target_commitish":"stable"},
			{"tag_name":"v1.0.0-stable", "target_commitish":"stable"}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		release, err := getLatestRelease(slug, "stable")
		if err != nil {
			t.Fatalf("getLatestRelease: %v", err)
		}

		// The branch loop returns the FIRST match in the array, so
		// v1.5.0-stable wins over v1.0.0-stable.
		if release.TagName != "v1.5.0-stable" {
			t.Errorf("TagName = %q, want v1.5.0-stable", release.TagName)
		}
	})

	t.Run("branch with no match returns error", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"tag_name":"v1.0.0", "target_commitish":"main"}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		_, err := getLatestRelease(slug, "nonexistent-branch")
		if err == nil {
			t.Error("expected error for branch with no matching release, got nil")
		}
	})
}

func TestGhUpdater_CheckUpdate(t *testing.T) {
	const ghSlug = "owner/repo"
	const releasesURL = "https://api.github.com/repos/" + ghSlug + "/releases"
	const installedTag = "v1.0.0"

	pack := core.Pack{
		Versions: map[string]string{"minecraft": "1.20.1"},
	}

	t.Run("mod without github update data gets a per-mod error", func(t *testing.T) {
		mod := &core.Mod{Name: "no-update", FileName: "test.jar"}

		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate returned overall error: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error, got %+v", results[0])
		}
	})

	t.Run("same tag yields UpdateAvailable=false", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"tag_name":"v1.0.0", "target_commitish":"main", "assets":[{"name":"asset.jar"}]}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		mod := newTestGithubMod(t, "Test", "asset.jar", ghSlug, installedTag, "", `^.+\.jar$`)

		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error != nil {
			t.Fatalf("unexpected per-mod error: %v", results[0].Error)
		}

		if results[0].UpdateAvailable {
			t.Errorf("expected UpdateAvailable=false, got %+v", results[0])
		}
	})

	t.Run("new tag with single matching asset yields UpdateAvailable=true", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"tag_name":"v2.0.0", "target_commitish":"main", "assets":[
				{"name":"asset-2.0.0.jar", "browser_download_url":"https://example.com/asset-2.0.0.jar"}
			]}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		mod := newTestGithubMod(t, "Test", "asset-1.0.0.jar", ghSlug, installedTag, "", `^.+\.jar$`)

		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error != nil {
			t.Fatalf("unexpected per-mod error: %v", results[0].Error)
		}

		if !results[0].UpdateAvailable {
			t.Fatalf("expected UpdateAvailable=true, got %+v", results[0])
		}

		want := "asset-1.0.0.jar -> asset-2.0.0.jar"
		if results[0].UpdateString != want {
			t.Errorf("UpdateString = %q, want %q", results[0].UpdateString, want)
		}
	})

	t.Run("new release with no assets yields per-mod error", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"tag_name":"v2.0.0", "target_commitish":"main", "assets":[]}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		mod := newTestGithubMod(t, "Test", "asset-1.0.0.jar", ghSlug, installedTag, "", `^.+\.jar$`)

		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error for no-assets case, got %+v", results[0])
		}
	})

	t.Run("new release with no regex-matching asset yields per-mod error", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"tag_name":"v2.0.0", "target_commitish":"main", "assets":[
				{"name":"checksums.txt"}
			]}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		// Regex matches only .jar files; the asset is .txt.
		mod := newTestGithubMod(t, "Test", "asset-1.0.0.jar", ghSlug, installedTag, "", `^.+\.jar$`)

		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error for no-regex-match case, got %+v", results[0])
		}
	})

	t.Run("new release with multiple regex-matching assets yields per-mod error", func(t *testing.T) {
		httpmock.Activate(t)

		body := `[
			{"tag_name":"v2.0.0", "target_commitish":"main", "assets":[
				{"name":"asset-a.jar"},
				{"name":"asset-b.jar"}
			]}
		]`

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(200, body))

		mod := newTestGithubMod(t, "Test", "asset-1.0.0.jar", ghSlug, installedTag, "", `^.+\.jar$`)

		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error for multiple-matches case, got %+v", results[0])
		}
	})

	t.Run("getLatestRelease error becomes a per-mod error", func(t *testing.T) {
		httpmock.Activate(t)

		httpmock.RegisterResponder("GET", releasesURL,
			httpmock.NewStringResponder(500, `{"message":"boom"}`))

		mod := newTestGithubMod(t, "Test", "asset-1.0.0.jar", ghSlug, installedTag, "", `^.+\.jar$`)

		results, err := ghUpdater{}.CheckUpdate([]*core.Mod{mod}, pack)
		if err != nil {
			t.Fatalf("CheckUpdate returned overall error: %v", err)
		}

		if results[0].Error == nil {
			t.Errorf("expected per-mod error from 500 response, got %+v", results[0])
		}
	})
}
