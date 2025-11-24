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
