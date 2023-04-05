package settings

import (
	"fmt"
	"github.com/packwiz/packwiz/cmdshared"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/unascribed/FlexVer/go/flexver"
	"golang.org/x/exp/slices"
	"os"
	"strings"
)

var acceptableVersionsCommand = &cobra.Command{
	Use:     "acceptable-versions",
	Short:   "Manage your pack's acceptable Minecraft versions. This must be a comma seperated list of Minecraft versions, e.g. 1.16.3,1.16.4,1.16.5",
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
		// Check if the list of versions is out of order, lowest to highest, and inform the user if it is
		// Compare the versions one by one to the next one, if the next one is lower, then it's out of order
		for i, v := range acceptableVersionsDeduped {
			if flexver.Less(acceptableVersionsDeduped[i+1], v) {
				fmt.Printf("Warning: Your acceptable versions list is out of order. ")
				// Give a do you mean example
				// Clone the list
				acceptableVersionsDedupedClone := make([]string, len(acceptableVersionsDeduped))
				copy(acceptableVersionsDedupedClone, acceptableVersionsDeduped)
				flexver.VersionSlice(acceptableVersionsDedupedClone).Sort()
				fmt.Printf("Did you mean %s?\n", strings.Join(acceptableVersionsDedupedClone, ", "))
				if cmdshared.PromptYesNo("Would you like to fix this automatically? [Y/n] ") {
					// If yes we'll just set the list to the sorted one
					acceptableVersionsDeduped = acceptableVersionsDedupedClone
					break
				} else {
					// If no we'll just continue
					break
				}
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
