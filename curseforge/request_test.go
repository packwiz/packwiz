package curseforge

import (
	"regexp"
	"testing"

	"github.com/jarcoal/httpmock"
)

func TestDecodeDefaultKey(t *testing.T) {
	// The default API key is a hardcoded base64 string at the top of
	// request.go. decodeDefaultKey should produce a non-empty result;
	// failure would panic (caught by the test runner).
	got := decodeDefaultKey()

	if got == "" {
		t.Fatal("decodeDefaultKey returned empty string")
	}

	// The decoded value should not still look like base64 — sanity
	// check that decoding actually happened.
	if got == cfApiKeyDefault {
		t.Errorf("decoded value equals the base64 input, decoding likely failed")
	}
}

func TestModFileInfoGetBestHash(t *testing.T) {
	mkHash := func(algo hashAlgo, value string) struct {
		Value     string   `json:"value"`
		Algorithm hashAlgo `json:"algo"`
	} {
		return struct {
			Value     string   `json:"value"`
			Algorithm hashAlgo `json:"algo"`
		}{Value: value, Algorithm: algo}
	}

	type hashEntry = struct {
		Value     string   `json:"value"`
		Algorithm hashAlgo `json:"algo"`
	}

	t.Run("falls back to murmur2 of Fingerprint when Hashes is nil", func(t *testing.T) {
		f := modFileInfo{Fingerprint: 4242}

		hash, format := f.getBestHash()

		if hash != "4242" || format != "murmur2" {
			t.Errorf("got (%q, %q), want (4242, murmur2)", hash, format)
		}
	})

	t.Run("falls back to murmur2 when Hashes is empty", func(t *testing.T) {
		f := modFileInfo{Fingerprint: 7, Hashes: []hashEntry{}}

		hash, format := f.getBestHash()

		if hash != "7" || format != "murmur2" {
			t.Errorf("got (%q, %q), want (7, murmur2)", hash, format)
		}
	})

	t.Run("md5 wins over murmur2", func(t *testing.T) {
		f := modFileInfo{
			Fingerprint: 4242,
			Hashes:      []hashEntry{mkHash(hashAlgoMD5, "md5-value")},
		}

		hash, format := f.getBestHash()

		if hash != "md5-value" || format != "md5" {
			t.Errorf("got (%q, %q), want (md5-value, md5)", hash, format)
		}
	})

	t.Run("sha1 wins over md5 and murmur2", func(t *testing.T) {
		f := modFileInfo{
			Fingerprint: 4242,
			Hashes: []hashEntry{
				mkHash(hashAlgoMD5, "md5-value"),
				mkHash(hashAlgoSHA1, "sha1-value"),
			},
		}

		hash, format := f.getBestHash()

		if hash != "sha1-value" || format != "sha1" {
			t.Errorf("got (%q, %q), want (sha1-value, sha1)", hash, format)
		}
	})

	t.Run("sha1 wins regardless of array order", func(t *testing.T) {
		f := modFileInfo{
			Hashes: []hashEntry{
				mkHash(hashAlgoSHA1, "sha1-value"),
				mkHash(hashAlgoMD5, "md5-value"),
			},
		}

		hash, format := f.getBestHash()

		if hash != "sha1-value" || format != "sha1" {
			t.Errorf("got (%q, %q), want (sha1-value, sha1)", hash, format)
		}
	})
}

func TestGetModInfo(t *testing.T) {
	t.Run("happy path returns the deserialized mod", func(t *testing.T) {
		httpmock.Activate(t)

		body := `{"data":{
			"id": 12345,
			"name": "Test Mod",
			"slug": "test-mod",
			"summary": "A mod used by TestGetModInfo",
			"gameId": 432,
			"classId": 6,
			"latestFiles": [
				{"id": 4567, "fileName": "test-mod-1.0.0.jar", "gameVersions": ["Fabric","1.20.1"]}
			],
			"latestFilesIndexes": [],
			"modLoaders": ["Fabric"],
			"links": {"websiteUrl": "https://www.curseforge.com/minecraft/mc-mods/test-mod"}
		}}`

		httpmock.RegisterResponder("GET", "https://api.curseforge.com/v1/mods/12345",
			httpmock.NewStringResponder(200, body))

		info, err := cfDefaultClient.getModInfo(12345)
		if err != nil {
			t.Fatalf("getModInfo: %v", err)
		}

		if info.ID != 12345 {
			t.Errorf("ID = %d, want 12345", info.ID)
		}

		if info.Name != "Test Mod" {
			t.Errorf("Name = %q, want %q", info.Name, "Test Mod")
		}

		if len(info.LatestFiles) != 1 || info.LatestFiles[0].ID != 4567 {
			t.Errorf("LatestFiles = %+v, want one entry with ID 4567", info.LatestFiles)
		}
	})

	t.Run("rejects response with mismatched ID", func(t *testing.T) {
		httpmock.Activate(t)

		// Server responds with a different ID than what was requested.
		body := `{"data":{"id": 999, "name": "Wrong"}}`
		httpmock.RegisterResponder("GET", "https://api.curseforge.com/v1/mods/12345",
			httpmock.NewStringResponder(200, body))

		_, err := cfDefaultClient.getModInfo(12345)
		if err == nil {
			t.Error("expected error for ID mismatch, got nil")
		}
	})

	t.Run("propagates non-200 status as error", func(t *testing.T) {
		httpmock.Activate(t)

		httpmock.RegisterResponder("GET", "https://api.curseforge.com/v1/mods/12345",
			httpmock.NewStringResponder(500, `{"error":"boom"}`))

		_, err := cfDefaultClient.getModInfo(12345)
		if err == nil {
			t.Error("expected error for 500 response, got nil")
		}
	})
}

func TestGetFileInfo(t *testing.T) {
	t.Run("happy path returns the deserialized file", func(t *testing.T) {
		httpmock.Activate(t)

		body := `{"data":{
			"id": 4567,
			"modId": 12345,
			"fileName": "test-mod-1.0.0.jar",
			"displayName": "Test Mod 1.0.0",
			"fileLength": 1024,
			"downloadUrl": "https://edge.forgecdn.net/files/4/5/test-mod.jar",
			"gameVersions": ["Fabric","1.20.1"],
			"fileFingerprint": 42424242,
			"hashes": [{"value":"sha1value","algo":1},{"value":"md5value","algo":2}]
		}}`

		httpmock.RegisterResponder("GET", "https://api.curseforge.com/v1/mods/12345/files/4567",
			httpmock.NewStringResponder(200, body))

		info, err := cfDefaultClient.getFileInfo(12345, 4567)
		if err != nil {
			t.Fatalf("getFileInfo: %v", err)
		}

		if info.ID != 4567 {
			t.Errorf("ID = %d, want 4567", info.ID)
		}

		if info.ModID != 12345 {
			t.Errorf("ModID = %d, want 12345", info.ModID)
		}

		if info.FileName != "test-mod-1.0.0.jar" {
			t.Errorf("FileName = %q, want %q", info.FileName, "test-mod-1.0.0.jar")
		}

		if info.Fingerprint != 42424242 {
			t.Errorf("Fingerprint = %d, want 42424242", info.Fingerprint)
		}

		// Verify the hash slice deserialized with both algorithm enum
		// values intact.
		if len(info.Hashes) != 2 {
			t.Fatalf("Hashes count = %d, want 2", len(info.Hashes))
		}
	})

	t.Run("rejects response with mismatched file ID", func(t *testing.T) {
		httpmock.Activate(t)

		body := `{"data":{"id": 999, "modId": 12345}}`
		httpmock.RegisterResponder("GET", "https://api.curseforge.com/v1/mods/12345/files/4567",
			httpmock.NewStringResponder(200, body))

		_, err := cfDefaultClient.getFileInfo(12345, 4567)
		if err == nil {
			t.Error("expected error for file ID mismatch, got nil")
		}
	})
}

func TestGetModInfoMultiple(t *testing.T) {
	httpmock.Activate(t)

	body := `{"data":[
		{"id": 1, "name": "A"},
		{"id": 2, "name": "B"},
		{"id": 3, "name": "C"}
	]}`

	httpmock.RegisterResponder("POST", "https://api.curseforge.com/v1/mods",
		httpmock.NewStringResponder(200, body))

	infos, err := cfDefaultClient.getModInfoMultiple([]uint32{1, 2, 3})
	if err != nil {
		t.Fatalf("getModInfoMultiple: %v", err)
	}

	if len(infos) != 3 {
		t.Fatalf("got %d infos, want 3", len(infos))
	}

	wantNames := []string{"A", "B", "C"}
	for i, info := range infos {
		if info.Name != wantNames[i] {
			t.Errorf("infos[%d].Name = %q, want %q", i, info.Name, wantNames[i])
		}
	}
}

func TestGetFileInfoMultiple(t *testing.T) {
	httpmock.Activate(t)

	body := `{"data":[
		{"id": 100, "modId": 1, "fileName": "a.jar"},
		{"id": 200, "modId": 2, "fileName": "b.jar"}
	]}`

	httpmock.RegisterResponder("POST", "https://api.curseforge.com/v1/mods/files",
		httpmock.NewStringResponder(200, body))

	infos, err := cfDefaultClient.getFileInfoMultiple([]uint32{100, 200})
	if err != nil {
		t.Fatalf("getFileInfoMultiple: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("got %d infos, want 2", len(infos))
	}

	if infos[0].FileName != "a.jar" || infos[1].FileName != "b.jar" {
		t.Errorf("got filenames %q / %q, want a.jar / b.jar",
			infos[0].FileName, infos[1].FileName)
	}
}

func TestGetSearch(t *testing.T) {
	httpmock.Activate(t)

	body := `{"data":[
		{"id": 1, "name": "Test Mod 1", "slug": "test-mod-1"},
		{"id": 2, "name": "Test Mod 2", "slug": "test-mod-2"}
	]}`

	// The query string varies (pageSize / gameId / slug / etc. can
	// appear in any order); match the path prefix.
	httpmock.RegisterRegexpResponder("GET",
		regexp.MustCompile(`^https://api\.curseforge\.com/v1/mods/search\?`),
		httpmock.NewStringResponder(200, body))

	results, err := cfDefaultClient.getSearch("test", "", 432, 6, 0, "1.20.1", modloaderTypeFabric)
	if err != nil {
		t.Fatalf("getSearch: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	if results[0].Slug != "test-mod-1" || results[1].Slug != "test-mod-2" {
		t.Errorf("got slugs %q / %q, want test-mod-1 / test-mod-2",
			results[0].Slug, results[1].Slug)
	}
}

func TestGetGames(t *testing.T) {
	httpmock.Activate(t)

	// The gameStatus and gameApiStatus enums are encoded as uints,
	// so we use numeric literals — gameStatusLive = 6, gameApiStatusPublic = 2.
	body := `{"data":[
		{"id": 432, "name": "Minecraft", "slug": "minecraft", "status": 6, "apiStatus": 2}
	]}`

	httpmock.RegisterResponder("GET",
		"https://api.curseforge.com/v1/games",
		httpmock.NewStringResponder(200, body))

	games, err := cfDefaultClient.getGames()
	if err != nil {
		t.Fatalf("getGames: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("got %d games, want 1", len(games))
	}

	g := games[0]
	if g.ID != 432 || g.Slug != "minecraft" {
		t.Errorf("game = %+v, want ID 432 / slug minecraft", g)
	}

	if g.Status != gameStatusLive {
		t.Errorf("Status = %d, want gameStatusLive (%d)", g.Status, gameStatusLive)
	}

	if g.APIStatus != gameApiStatusPublic {
		t.Errorf("APIStatus = %d, want gameApiStatusPublic (%d)", g.APIStatus, gameApiStatusPublic)
	}
}

func TestGetCategories(t *testing.T) {
	httpmock.Activate(t)

	body := `{"data":[
		{"id": 6, "slug": "mc-mods", "isClass": true, "classId": 0},
		{"id": 12, "slug": "resource-packs", "isClass": true, "classId": 0},
		{"id": 423, "slug": "magic", "isClass": false, "classId": 6}
	]}`

	httpmock.RegisterResponder("GET",
		"https://api.curseforge.com/v1/categories?gameId=432",
		httpmock.NewStringResponder(200, body))

	cats, err := cfDefaultClient.getCategories(432)
	if err != nil {
		t.Fatalf("getCategories: %v", err)
	}

	if len(cats) != 3 {
		t.Fatalf("got %d categories, want 3", len(cats))
	}

	// Verify the class / non-class distinction round-trips.
	if !cats[0].IsClass || cats[0].ClassID != 0 {
		t.Errorf("first entry should be a class with classId=0; got %+v", cats[0])
	}

	if cats[2].IsClass || cats[2].ClassID != 6 {
		t.Errorf("last entry should be a non-class under classId=6; got %+v", cats[2])
	}
}

func TestGetFingerprintInfo(t *testing.T) {
	httpmock.Activate(t)

	// Body covers the three branches detect.go cares about: an exact
	// match (with the embedded file and a latestFiles slice), an
	// exact-fingerprints index, and a list of unmatched fingerprints.
	body := `{"data":{
		"isCacheBuilt": true,
		"exactMatches": [
			{
				"id": 12345,
				"file": {"id": 4567, "modId": 12345, "fileName": "matched.jar", "fileFingerprint": 1111111111},
				"latestFiles": []
			}
		],
		"exactFingerprints": [1111111111],
		"partialMatches": [],
		"partialMatchFingerprints": {},
		"installedFingerprints": [1111111111, 2222222222],
		"unmatchedFingerprints": [2222222222]
	}}`

	httpmock.RegisterResponder("POST",
		"https://api.curseforge.com/v1/fingerprints",
		httpmock.NewStringResponder(200, body))

	resp, err := cfDefaultClient.getFingerprintInfo([]uint32{1111111111, 2222222222})
	if err != nil {
		t.Fatalf("getFingerprintInfo: %v", err)
	}

	if !resp.IsCacheBuilt {
		t.Error("IsCacheBuilt = false, want true")
	}

	if len(resp.ExactMatches) != 1 {
		t.Fatalf("ExactMatches count = %d, want 1", len(resp.ExactMatches))
	}

	match := resp.ExactMatches[0]
	if match.ID != 12345 || match.File.ID != 4567 || match.File.FileName != "matched.jar" {
		t.Errorf("unexpected exact match: %+v", match)
	}

	if len(resp.UnmatchedFingerprints) != 1 || resp.UnmatchedFingerprints[0] != 2222222222 {
		t.Errorf("UnmatchedFingerprints = %v, want [2222222222]", resp.UnmatchedFingerprints)
	}
}
