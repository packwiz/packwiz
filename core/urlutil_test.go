package core

import (
	"testing"
)

func TestReencodeURL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain URL passes through unchanged",
			input: "https://example.com/path/file.jar",
			want:  "https://example.com/path/file.jar",
		},
		{
			name:  "square brackets are encoded to %5B and %5D",
			input: "https://edge.forgecdn.net/files/123/456/mod[1.0].jar",
			want:  "https://edge.forgecdn.net/files/123/456/mod%5B1.0%5D.jar",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReencodeURL(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Errorf("ReencodeURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
