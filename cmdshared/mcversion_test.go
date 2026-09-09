package cmdshared

import (
	"testing"
	"time"
)

// CheckValid calls os.Exit(1) on miss, so only the success path is
// testable here. Calling the failure path would terminate the test
// binary. See .claude/TODO.md ("Audit and improve error handling")
// for the broader refactor — these helpers should return errors
// rather than calling os.Exit directly.

func TestMcVersionManifestCheckValid_Success(t *testing.T) {
	type versionEntry = struct {
		ID          string    `json:"id"`
		Type        string    `json:"type"`
		URL         string    `json:"url"`
		Time        time.Time `json:"time"`
		ReleaseTime time.Time `json:"releaseTime"`
	}

	m := McVersionManifest{
		Versions: []versionEntry{
			{ID: "1.19.4"},
			{ID: "1.20.1"},
			{ID: "1.20.2"},
		},
	}

	// Each of these is present in the manifest; CheckValid should
	// return silently. A regression would terminate the test process.
	for _, version := range []string{"1.19.4", "1.20.1", "1.20.2"} {
		t.Run(version, func(t *testing.T) {
			m.CheckValid(version)
		})
	}
}
