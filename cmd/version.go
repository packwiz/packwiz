package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// rejectPreReleaseBuild returns an error if the canonical semver string
// contains pre-release or build metadata. Both set and bump share this
// policy so it lives in one place.
func rejectPreReleaseBuild(requested string, s string) error {
	if strings.Contains(s, "-") || strings.Contains(s, "+") {
		return fmt.Errorf("'%s' is not allowed: pre-release tags and build metadata are not supported. Use MAJOR.MINOR.PATCH (e.g. 1.2.3).", requested)
	}
	return nil
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Get or set the modpack version",
	Long: `Get or set the version field in pack.toml.

Use 'packwiz version get' to read the current version,
'packwiz version set <ver>' to set an exact version,
or 'packwiz version bump [major|minor|patch]' to increment semver.

After changing the version, run 'packwiz refresh' to regenerate
index.toml hashes, then commit and push to update your pack.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: act like 'get'
		versionGetCmd.Run(versionGetCmd, args)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.AddCommand(versionGetCmd)
	versionCmd.AddCommand(versionSetCmd)
	versionCmd.AddCommand(versionBumpCmd)
}
