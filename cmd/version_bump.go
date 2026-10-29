package cmd

import (
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

// runVersionBump performs the version-bump logic and returns (previous, new, error).
func runVersionBump(pack *core.Pack, level string) (string, string, error) {
	if pack.Version == "" {
		return "", "", fmt.Errorf("no version is set in pack.toml, nothing to bump")
	}

	v, err := semver.StrictNewVersion(pack.Version)
	if err != nil {
		return "", "", fmt.Errorf("current version '%s' is not valid semver: %s", pack.Version, err)
	}
	s := v.String()
	if err := rejectPreReleaseBuild(pack.Version, s); err != nil {
		return "", "", err
	}

	previous := s
	var newStr string
	switch level {
	case "major":
		newStr = v.IncMajor().String()
	case "minor":
		newStr = v.IncMinor().String()
	case "patch":
		newStr = v.IncPatch().String()
	default:
		return "", "", fmt.Errorf("invalid bump level '%s'", level)
	}
	v, err = semver.StrictNewVersion(newStr)
	if err != nil {
		// Should not happen: we already validated the current version.
		return "", "", fmt.Errorf("failed to compute bumped version: %s", err)
	}

	pack.Version = v.String()
	if err := pack.Write(); err != nil {
		return "", "", fmt.Errorf("failed to write pack.toml: %w", err)
	}
	return previous, v.String(), nil
}

var versionBumpCmd = &cobra.Command{
	Use:   "bump [major|minor|patch]",
	Short: "Increment the modpack version",
	Long: `Increment the modpack version following semver rules.

Defaults to 'patch' if no level is given. Requires a valid current
version in pack.toml.

After changing the version, run 'packwiz refresh' to regenerate
index.toml hashes, then commit and push to update your pack.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		level := "patch"
		if len(args) == 1 {
			level = args[0]
		}

		switch level {
		case "major", "minor", "patch":
			// valid
		default:
			fmt.Fprintln(os.Stderr, "Error: invalid bump level '"+level+"'.")
			fmt.Fprintln(os.Stderr, "Hint: use 'major', 'minor', or 'patch' (default: patch).")
			fmt.Fprintln(os.Stderr, "Example: 'packwiz version bump minor'")
			os.Exit(1)
		}

		pack, err := core.LoadPack()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "Hint: run 'packwiz init' in this directory first, or check you're in the modpack folder.")
			os.Exit(1)
		}

		previous, newVersion, err := runVersionBump(&pack, level)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "Hint: set a valid version first with 'packwiz version set 1.0.0'")
			os.Exit(1)
		}

		fmt.Printf("Version bumped %s: %s -> %s\n", level, previous, newVersion)
		fmt.Println("Next: run 'packwiz refresh' to regenerate index.toml hashes, then commit and push.")
	},
}
