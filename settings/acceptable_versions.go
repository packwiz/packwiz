package settings

import (
	"fmt"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"os"
	"strings"
)

var acceptableVersionsCommand = &cobra.Command{
	Use:     "acceptable-versions",
	Short:   "Manage your pack's acceptable versions. This must be a comma seperated list of Minecraft versions, e.g. 1.16.5,1.16.4,1.16.3",
	Aliases: []string{"av"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modpack, err := core.LoadPack()
		if err != nil {
			// Check if it's a no such file or directory error
			if os.IsNotExist(err) {
				fmt.Println("No pack.toml file found, run 'packwiz init' to create one!")
				os.Exit(1)
			}
			fmt.Printf("Error loading pack: %s\n", err)
			os.Exit(1)
		}
		acceptableVersions := args[0]
		acceptableVersionsList := strings.Split(acceptableVersions, ",")
		// Dedupe the list
		acceptableVersionsDeduped := []string(nil)
		for i, v := range acceptableVersionsList {
			if !slices.Contains(acceptableVersionsList[i+1:], v) {
				acceptableVersionsDeduped = append(acceptableVersionsDeduped, v)
			}
		}
		modpack.Options["acceptable-game-versions"] = acceptableVersionsDeduped
		err = modpack.Write()
		if err != nil {
			fmt.Printf("Error writing pack: %s\n", err)
			os.Exit(1)
		}
		// Print success message
		prettyList := strings.Join(acceptableVersionsDeduped, ", ")
		fmt.Printf("Set acceptable versions to %s\n", prettyList)
	},
}

func init() {
	settingsCmd.AddCommand(acceptableVersionsCommand)
}
