package cmdshared

import "testing"

func TestGetRawForgeVersion(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no dash returns input unchanged",
			input: "47.1.0",
			want:  "47.1.0",
		},
		{
			name:  "mcVersion-loaderVersion strips the mcVersion prefix",
			input: "1.20.1-47.1.0",
			want:  "47.1.0",
		},
		{
			// Versions can include further dashes (rc / beta tails); the
			// function uses Split(...)[1], so it returns only the segment
			// between the first and second dash. Pinning this behavior;
			// callers that expect the full tail need a different parser.
			name:  "second dash truncates the version tail",
			input: "1.20.1-47.1.0-rc1",
			want:  "47.1.0",
		},
		{
			name:  "empty input passes through",
			input: "",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetRawForgeVersion(tc.input); got != tc.want {
				t.Errorf("GetRawForgeVersion(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
