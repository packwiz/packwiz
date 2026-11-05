package github

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/mitchellh/mapstructure"
)

func TestFetchRepo(t *testing.T) {
	t.Run("happy path returns the deserialized repo", func(t *testing.T) {
		httpmock.Activate(t)

		body := `{
			"id": 12345,
			"name": "packwiz",
			"full_name": "packwiz/packwiz"
		}`

		httpmock.RegisterResponder("GET",
			"https://api.github.com/repos/packwiz/packwiz",
			httpmock.NewStringResponder(200, body))

		repo, err := fetchRepo("packwiz/packwiz")
		if err != nil {
			t.Fatalf("fetchRepo: %v", err)
		}

		if repo.ID != 12345 || repo.Name != "packwiz" || repo.FullName != "packwiz/packwiz" {
			t.Errorf("got %+v, want ID=12345 Name=packwiz FullName=packwiz/packwiz", repo)
		}
	})

	t.Run("rejects response with empty full_name", func(t *testing.T) {
		httpmock.Activate(t)

		// GitHub returns 200 OK with a partial body in some edge
		// cases (e.g., the slug points at a path that isn't actually
		// a repo). The function guards by requiring FullName to be
		// non-empty.
		httpmock.RegisterResponder("GET",
			"https://api.github.com/repos/missing/repo",
			httpmock.NewStringResponder(200, `{}`))

		_, err := fetchRepo("missing/repo")
		if err == nil {
			t.Error("expected error for empty FullName, got nil")
		}
	})

	t.Run("propagates non-200 status as error", func(t *testing.T) {
		httpmock.Activate(t)

		httpmock.RegisterResponder("GET",
			"https://api.github.com/repos/nope/nope",
			httpmock.NewStringResponder(404, `{"message":"Not Found"}`))

		_, err := fetchRepo("nope/nope")
		if err == nil {
			t.Error("expected error for 404 response, got nil")
		}
	})
}

func TestGhUpdateData_ToMap_RoundTrip(t *testing.T) {
	original := ghUpdateData{
		Slug:   "packwiz/packwiz",
		Tag:    "v1.0.0",
		Branch: "main",
		Regex:  `.*\.jar$`,
	}

	m, err := original.ToMap()
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}

	var back ghUpdateData
	if err := mapstructure.Decode(m, &back); err != nil {
		t.Fatalf("mapstructure.Decode: %v", err)
	}

	if !reflect.DeepEqual(back, original) {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", back, original)
	}
}

func TestAsset_GetSha256(t *testing.T) {
	httpmock.Activate(t)

	body := "binary jar contents would normally go here"

	// Pre-compute the expected hash via the standard library so the
	// test isn't pinning a copy-paste constant.
	sum := sha256.Sum256([]byte(body))
	want := hex.EncodeToString(sum[:])

	const downloadURL = "https://github.com/owner/repo/releases/download/v1.0.0/asset.jar"

	httpmock.RegisterResponder("GET", downloadURL,
		httpmock.NewStringResponder(200, body))

	asset := Asset{BrowserDownloadURL: downloadURL, Name: "asset.jar"}

	got, err := asset.getSha256()
	if err != nil {
		t.Fatalf("getSha256: %v", err)
	}

	if got != want {
		t.Errorf("getSha256 = %q, want %q", got, want)
	}
}
