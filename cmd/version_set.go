package cmd

import (
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

// runVersionSet performs the version-set logic and returns the new version on success.
func runVersionSet(pack *core.Pack, requested string) (string, error) {
	v, err := semver.StrictNewVersion(requested)
	if err != nil {
		return "", fmt.Errorf("'%s' is not a valid semantic version: use MAJOR.MINOR.PATCH (e.g. 1.2.3)", requested)
	}
	s := v.String()
	if err := rejectPreReleaseBuild(requested, s); err != nil {
		return "", err
	}

	pack.Version = s
	if err := pack.Write(); err != nil {
		return "", fmt.Errorf("failed to write pack.toml: %w", err)
	}
	return s, nil
}

var versionSetCmd = &cobra.Command{
	Use:   "set <version>",
	Short: "Set the modpack version to an exact semver",
	Long: `Set the version field in pack.toml to an exact semantic version.

The version must be a valid semver string (MAJOR.MINOR.PATCH, e.g. 1.2.3).
Pre-release and build metadata are not supported.

After changing the version, run 'packwiz refresh' to regenerate
index.toml hashes, then commit and push to update your pack.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requested := args[0]

		pack, err := core.LoadPack()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "Hint: run 'packwiz init' in this directory first, or check you're in the modpack folder.")
			os.Exit(1)
		}

		newVersion, err := runVersionSet(&pack, requested)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Printf("Version set to %s\n", newVersion)
		fmt.Println("Next: run 'packwiz refresh' to regenerate index.toml hashes, then commit and push.")
	},
}
