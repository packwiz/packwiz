package core

import (
	"reflect"
	"slices"
	"testing"
)

func TestSelectPreferredHash(t *testing.T) {
	cases := []struct {
		name       string
		hashes     map[string]string
		wantFormat string
		wantHash   string
	}{
		{
			name:       "empty map yields empty selection",
			hashes:     map[string]string{},
			wantFormat: "",
			wantHash:   "",
		},
		{
			name:       "only murmur2 present",
			hashes:     map[string]string{"murmur2": "1234"},
			wantFormat: "murmur2",
			wantHash:   "1234",
		},
		{
			name:       "only sha256 present",
			hashes:     map[string]string{"sha256": "abc"},
			wantFormat: "sha256",
			wantHash:   "abc",
		},
		{
			// preferredHashList iterates murmur2 → md5 → sha1 → sha256
			// → sha512. The function overwrites currHash on each match,
			// so the last (strongest) match wins.
			name: "strongest hash wins when several present",
			hashes: map[string]string{
				"murmur2": "1",
				"sha1":    "2",
				"sha512":  "3",
			},
			wantFormat: "sha512",
			wantHash:   "3",
		},
		{
			name:       "unknown formats are ignored",
			hashes:     map[string]string{"blake3": "x"},
			wantFormat: "",
			wantHash:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFormat, gotHash := selectPreferredHash(tc.hashes)

			if gotFormat != tc.wantFormat || gotHash != tc.wantHash {
				t.Errorf("got (%q, %q), want (%q, %q)",
					gotFormat, gotHash, tc.wantFormat, tc.wantHash)
			}
		})
	}
}

func TestGetHashListsForDownload(t *testing.T) {
	t.Run("validate format matches cache format and no extras", func(t *testing.T) {
		// cacheHashFormat is "sha256" — when validation is also sha256,
		// the returned hashesToObtain list should be empty.
		extras, validateMap := getHashListsForDownload(nil, "sha256", "abc")

		if len(extras) != 0 {
			t.Errorf("extras = %v, want empty", extras)
		}

		if !reflect.DeepEqual(validateMap, map[string]string{"sha256": "abc"}) {
			t.Errorf("validateMap = %v, want {sha256: abc}", validateMap)
		}
	})

	t.Run("validate format differs from cache format", func(t *testing.T) {
		// When validating with md5, the cache still wants sha256, so
		// sha256 ends up in the extras list.
		extras, validateMap := getHashListsForDownload(nil, "md5", "xyz")

		if !slices.Contains(extras, "sha256") {
			t.Errorf("extras = %v, want it to contain sha256", extras)
		}

		if validateMap["md5"] != "xyz" {
			t.Errorf("validateMap[md5] = %q, want xyz", validateMap["md5"])
		}
	})

	t.Run("hashesToObtain deduplicates against validate and cache formats", func(t *testing.T) {
		// Asking for sha256 + sha512 while validating with sha256 (and
		// cache also sha256) should yield only sha512 in extras.
		extras, _ := getHashListsForDownload([]string{"sha256", "sha512"}, "sha256", "abc")

		if !reflect.DeepEqual(extras, []string{"sha512"}) {
			t.Errorf("extras = %v, want [sha512]", extras)
		}
	})
}
