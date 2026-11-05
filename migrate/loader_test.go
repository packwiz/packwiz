package migrate

import (
	"testing"

	"github.com/packwiz/packwiz/core"
)

// validateVersion and the surrounding loader command both call
// os.Exit(1) on failure paths, so only the success paths are
// exercised here. See .claude/TODO.md ("Audit and improve error
// handling") for the broader refactor — these helpers should return
// errors rather than calling os.Exit directly.

func TestValidateVersion_Success(t *testing.T) {
	versions := []string{"0.14.21", "0.15.0", "0.15.3"}
	loader := core.ModLoaderComponent{Name: "fabric", FriendlyName: "Fabric"}

	for _, v := range versions {
		t.Run(v, func(t *testing.T) {
			// Should return silently. A regression would terminate
			// the test process via os.Exit(1).
			validateVersion(versions, v, loader)
		})
	}
}

func TestUpdatePackToVersion(t *testing.T) {
	loader := core.ModLoaderComponent{Name: "fabric", FriendlyName: "Fabric"}

	t.Run("new version replaces the old one and returns true", func(t *testing.T) {
		pack := core.Pack{
			Versions: map[string]string{"fabric": "0.14.21", "minecraft": "1.20.1"},
		}

		ok := updatePackToVersion("0.15.3", pack, loader)
		if !ok {
			t.Fatal("expected true when version differed, got false")
		}

		// The function takes pack by value, but Versions is a map
		// (reference semantics), so the mutation is visible to the
		// caller.
		if got := pack.Versions["fabric"]; got != "0.15.3" {
			t.Errorf("pack.Versions[fabric] = %q, want %q", got, "0.15.3")
		}

		// Unrelated entries are untouched.
		if got := pack.Versions["minecraft"]; got != "1.20.1" {
			t.Errorf("pack.Versions[minecraft] = %q, want %q (should be untouched)", got, "1.20.1")
		}
	})

	t.Run("identical version is a no-op and returns false", func(t *testing.T) {
		pack := core.Pack{
			Versions: map[string]string{"fabric": "0.15.3"},
		}

		ok := updatePackToVersion("0.15.3", pack, loader)
		if ok {
			t.Errorf("expected false when version already matched, got true")
		}

		if got := pack.Versions["fabric"]; got != "0.15.3" {
			t.Errorf("pack.Versions[fabric] = %q, want %q (should be unchanged)", got, "0.15.3")
		}
	})
}
