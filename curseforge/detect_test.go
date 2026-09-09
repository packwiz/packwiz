package curseforge

import (
	"bytes"
	"testing"
)

func TestIsWhitespaceCharacter(t *testing.T) {
	cases := []struct {
		name string
		b    byte
		want bool
	}{
		{"tab (9)", 9, true},
		{"line feed (10)", 10, true},
		{"carriage return (13)", 13, true},
		{"space (32)", 32, true},
		{"vertical tab (11) is not in the set", 11, false},
		{"form feed (12) is not in the set", 12, false},
		{"NUL (0)", 0, false},
		{"letter 'a'", 'a', false},
		{"digit '0'", '0', false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWhitespaceCharacter(tc.b); got != tc.want {
				t.Errorf("isWhitespaceCharacter(%d) = %v, want %v", tc.b, got, tc.want)
			}
		})
	}
}

func TestComputeNormalizedArray(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "input without whitespace passes through",
			input: []byte("abcdef"),
			want:  []byte("abcdef"),
		},
		{
			name:  "spaces are stripped",
			input: []byte("a b c"),
			want:  []byte("abc"),
		},
		{
			name:  "all four normalised whitespace characters are stripped",
			input: []byte{'a', 9, 'b', 10, 'c', 13, 'd', 32, 'e'},
			want:  []byte("abcde"),
		},
		{
			name:  "vertical tab and form feed are preserved (not in the set)",
			input: []byte{'a', 11, 'b', 12, 'c'},
			want:  []byte{'a', 11, 'b', 12, 'c'},
		},
		{
			name:  "empty input yields nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "all-whitespace input yields nil",
			input: []byte{9, 10, 13, 32},
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeNormalizedArray(tc.input)

			if !bytes.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetByteArrayHash(t *testing.T) {
	// Regression pin: the murmur2 fingerprint of "abc" (no whitespace to
	// strip) under seed 1. The exact value is whatever the upstream
	// go-murmur library currently produces; pinning it catches changes
	// in either go-murmur or our normalization pipeline.
	const want uint32 = 1621425345
	got := getByteArrayHash([]byte("abc"))

	if got != want {
		t.Errorf("getByteArrayHash([]byte(\"abc\")) = %d, want %d", got, want)
	}
}

func TestGetByteArrayHash_NormalizesWhitespace(t *testing.T) {
	// The function strips normalized whitespace before hashing, so
	// these two inputs must produce identical fingerprints — the
	// property that makes CurseForge fingerprint matching robust to
	// line-ending differences between platforms.
	a := getByteArrayHash([]byte("hello world"))
	b := getByteArrayHash([]byte("hello\nworld"))
	c := getByteArrayHash([]byte("hel l o\tw\rorld"))

	if a == 0 {
		t.Fatal("hash of non-empty input should not be 0")
	}

	if a != b || a != c {
		t.Errorf("expected matching hashes after whitespace normalization, got %d / %d / %d", a, b, c)
	}
}
