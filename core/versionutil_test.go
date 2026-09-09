package core

import (
	"embed"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
)

// For reproducability, we store a list of sample xml files for various endpoints
// these have been edited slightly to cut down on the number of entries, but are
// otherwise taken from the endpoints themselves

//go:embed version_test_files/*
var versionTestFiles embed.FS

func registerMock(url string, filename string) {
	bytes, err := versionTestFiles.ReadFile("version_test_files/" + filename)
	if err != nil {
		println("Error " + filename + " not in version_test_files/")
		os.Exit(1)
	}
	httpmock.RegisterResponder("GET", url, httpmock.NewBytesResponder(200, bytes))
}

func queryWithMock(t *testing.T, q VersionListQuery) *ModLoaderVersions {
	httpmock.Activate(t)

	registerMock("https://maven.fabricmc.net/net/fabricmc/fabric-loader/maven-metadata.xml", "fabric.xml")
	registerMock("https://repo.mumfrey.com/content/repositories/snapshots/com/mumfrey/liteloader/maven-metadata.xml", "liteloader.xml")
	registerMock("https://maven.quiltmc.org/repository/release/org/quiltmc/quilt-loader/maven-metadata.xml", "quilt.xml")
	registerMock("https://files.minecraftforge.net/maven/net/minecraftforge/forge/maven-metadata.xml", "forge.xml")
	registerMock("https://maven.neoforged.net/releases/net/neoforged/forge/maven-metadata.xml", "neoforge_old.xml")
	registerMock("https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml", "neoforge.xml")

	versionData, err := DoQuery(q)

	if err != nil {
		t.Logf("Error fetching versions for %s: %s", q.Loader.FriendlyName, err)
		if strings.Contains(err.Error(), "no responder found") {
			t.Log("You likely need to register a mock for this url")
		}
		t.FailNow()
	}

	return versionData
}

func expectLatest(t *testing.T, loader string, version string, expectedLatest string) string {
	loaderData, ok := ModLoaders[loader]
	if !ok {
		t.Fatal("Could not find loader")
	}
	versionData := queryWithMock(t, MakeQuery(loaderData, version))

	if len(versionData.Versions) == 0 {
		t.Error("There should be at least one version")
	}
	if versionData.Latest != expectedLatest {
		t.Errorf("Expected latest version to be %s, found %s", expectedLatest, versionData.Latest)
	}

	return versionData.Latest
}

func expectValid(t *testing.T, loader string, version string, expectedValid string) {
	loaderData, ok := ModLoaders[loader]
	if !ok {
		t.Fatal("Could not find loader")
	}
	versionData := queryWithMock(t, MakeQuery(loaderData, version))

	if !slices.Contains(versionData.Versions, expectedValid) {
		t.Errorf("Expected %s to be a valid version for %s. Valid versions:\n%s", expectedValid, loaderData.FriendlyName, versionData.Versions)
	}
}

func expectInvalid(t *testing.T, loader string, version string, expectedValid string) {
	loaderData, ok := ModLoaders[loader]
	if !ok {
		t.Fatal("Could not find loader")
	}
	versionData := queryWithMock(t, MakeQuery(loaderData, version))

	if slices.Contains(versionData.Versions, expectedValid) {
		t.Errorf("Expected %s not to be a valid version for %s. Valid versions:\n%s", expectedValid, loaderData.FriendlyName, versionData.Versions)
	}
}

func TestFabric121(t *testing.T) {
	expectLatest(t, "fabric", "1.21", "0.17.3")
}

func TestFabric010Valid(t *testing.T) {
	expectValid(t, "fabric", "1.21", "0.10.6+build.214")
}

func TestQuilt121(t *testing.T) {
	expectLatest(t, "quilt", "1.21", "0.29.3-beta.1")
}

func TestForge121(t *testing.T) {
	expectLatest(t, "forge", "1.21", "51.0.33")
}

func TestLiteLoader112(t *testing.T) {
	expectLatest(t, "liteloader", "1.12", "1.12-SNAPSHOT")
}

func TestNeoForge1201(t *testing.T) {
	expectLatest(t, "neoforge", "1.20.1", "47.1.106")
}

func TestNeoForge121(t *testing.T) {
	expectLatest(t, "neoforge", "1.21", "21.0.167")
}

func TestNeoForge1211(t *testing.T) {
	expectLatest(t, "neoforge", "1.21.1", "21.1.213")
	expectValid(t, "neoforge", "1.21.1", "21.1.201")
	expectInvalid(t, "neoforge", "1.21.1", "21.10.43-beta")
}

func TestNeoForge1210(t *testing.T) {
	expectLatest(t, "neoforge", "1.21.10", "21.10.43-beta")
}

func TestNeoForge261snapshot6(t *testing.T) {
	expectLatest(t, "neoforge", "26.1-snapshot-6", "26.1.0.0-alpha.10+snapshot-6")
	expectValid(t, "neoforge", "26.1-snapshot-6", "26.1.0.0-alpha.9+snapshot-6")
	expectInvalid(t, "neoforge", "26.1-snapshot-6", "26.1.0.0-alpha.11+snapshot-7")
}

func TestHighestSliceIndex(t *testing.T) {
	cases := []struct {
		name   string
		slice  []string
		values []string
		want   int
	}{
		{
			name:   "no overlap returns -1",
			slice:  []string{"1.19", "1.20"},
			values: []string{"1.18"},
			want:   -1,
		},
		{
			name:   "single match returns its index",
			slice:  []string{"1.19", "1.20"},
			values: []string{"1.20"},
			want:   1,
		},
		{
			// values that don't exist in slice are ignored; the highest
			// index of those that DO exist wins.
			name:   "mix of present and absent values takes the highest present",
			slice:  []string{"1.19", "1.20"},
			values: []string{"1.20", "1.18"},
			want:   1,
		},
		{
			name:   "all present picks highest",
			slice:  []string{"1.19", "1.20", "1.21"},
			values: []string{"1.19", "1.20", "1.21"},
			want:   2,
		},
		{
			name:   "empty values returns -1",
			slice:  []string{"1.19", "1.20"},
			values: nil,
			want:   -1,
		},
		{
			name:   "empty slice returns -1",
			slice:  nil,
			values: []string{"1.19"},
			want:   -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HighestSliceIndex(tc.slice, tc.values); got != tc.want {
				t.Errorf("HighestSliceIndex(%v, %v) = %d, want %d",
					tc.slice, tc.values, got, tc.want)
			}
		})
	}
}

func TestVersionListQuery_WithQueryType(t *testing.T) {
	// Builder method: copies the receiver, replaces QueryType, leaves
	// Loader and McVersion intact. Verifies it doesn't mutate the
	// original.
	original := VersionListQuery{
		Loader:    ModLoaderComponent{Name: "fabric", FriendlyName: "Fabric"},
		McVersion: "1.20.1",
		QueryType: Latest,
	}

	updated := original.WithQueryType(Recommended)

	if updated.QueryType != Recommended {
		t.Errorf("updated QueryType = %v, want Recommended", updated.QueryType)
	}

	if updated.Loader.Name != "fabric" || updated.McVersion != "1.20.1" {
		t.Errorf("WithQueryType lost other fields: got %+v", updated)
	}

	if original.QueryType != Latest {
		t.Errorf("WithQueryType mutated the receiver: original.QueryType = %v, want Latest", original.QueryType)
	}
}

func TestComponentToFriendlyName(t *testing.T) {
	cases := []struct {
		name      string
		component string
		want      string
	}{
		{"minecraft is a hardcoded special case", "minecraft", "Minecraft"},
		{"fabric maps to its FriendlyName", "fabric", "Fabric loader"},
		{"neoforge maps to its FriendlyName", "neoforge", "NeoForge"},
		{"forge maps to its FriendlyName", "forge", "Forge"},
		{"unknown component returns input unchanged", "definitely-not-a-loader", "definitely-not-a-loader"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComponentToFriendlyName(tc.component); got != tc.want {
				t.Errorf("ComponentToFriendlyName(%q) = %q, want %q", tc.component, got, tc.want)
			}
		})
	}
}
