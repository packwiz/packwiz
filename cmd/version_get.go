package cmd

import (
	"fmt"
	"os"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

// runVersionGet performs the version-get logic and returns the current version on success.
func runVersionGet(pack *core.Pack) (string, error) {
	if pack.Version == "" {
		return "", fmt.Errorf("no version is set in pack.toml")
	}
	return pack.Version, nil
}

var versionGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Print the current modpack version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "Hint: run 'packwiz init' in this directory first, or check you're in the modpack folder.")
			os.Exit(1)
		}

		version, err := runVersionGet(&pack)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "Hint: set one with 'packwiz version set 1.0.0'")
			os.Exit(1)
		}
		fmt.Println(version)
	},
}
