package settings

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/packwiz/packwiz/cmdshared"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/unascribed/FlexVer/go/flexver"
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
		var currentVersions []string
		// Check if they have no options whatsoever
		if modpack.Options == nil {
			// Initialize the options
			modpack.Options = make(map[string]interface{})
		}
		// Check if the acceptable-game-versions is nil, which would mean their pack.toml doesn't have it set yet
		if modpack.Options["acceptable-game-versions"] != nil {
			// Convert the interface{} to a string slice
			for _, v := range modpack.Options["acceptable-game-versions"].([]interface{}) {
				currentVersions = append(currentVersions, v.(string))
			}
		}
		// Check our flags to see if we're adding or removing
		if flagAdd {
			acceptableVersion := args[0]
			// Check if the version is already in the list
			if slices.Contains(currentVersions, acceptableVersion) {
				fmt.Printf("Version %s is already in your acceptable versions list!\n", acceptableVersion)
				os.Exit(1)
			}
			// Add the version to the list and re-sort it
			currentVersions = append(currentVersions, acceptableVersion)
			flexver.VersionSlice(currentVersions).Sort()
			// Set the new list
			modpack.Options["acceptable-game-versions"] = currentVersions
			// Save the pack
			err = modpack.Write()
			if err != nil {
				fmt.Printf("Error writing pack: %s\n", err)
				os.Exit(1)
			}
			// Print success message
			prettyList := strings.Join(currentVersions, ", ")
			prettyList += ", " + modpack.Versions["minecraft"]
			fmt.Printf("Added %s to acceptable versions list, now %s\n", acceptableVersion, prettyList)
		} else if flagRemove {
			acceptableVersion := args[0]
			// Check if the version is in the list
			if !slices.Contains(currentVersions, acceptableVersion) {
				fmt.Printf("Version %s is not in your acceptable versions list!\n", acceptableVersion)
				os.Exit(1)
			}
			// Remove the version from the list
			i := slices.Index(currentVersions, acceptableVersion)
			currentVersions = slices.Delete(currentVersions, i, i+1)
			// Sort it just in case it's out of order
			flexver.VersionSlice(currentVersions).Sort()
			// Set the new list
			modpack.Options["acceptable-game-versions"] = currentVersions
			// Save the pack
			err = modpack.Write()
			if err != nil {
				fmt.Printf("Error writing pack: %s\n", err)
				os.Exit(1)
			}
			// Print success message
			prettyList := strings.Join(currentVersions, ", ")
			prettyList += ", " + modpack.Versions["minecraft"]
			fmt.Printf("Removed %s from acceptable versions list, now %s\n", acceptableVersion, prettyList)
		} else {
			// Overwriting
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
			// If it's only 1 element long, then it's already sorted
			if len(acceptableVersionsDeduped) > 1 {
				for i, v := range acceptableVersionsDeduped {
					if i+1 < len(acceptableVersionsDeduped) && flexver.Less(acceptableVersionsDeduped[i+1], v) {
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
			}
			modpack.Options["acceptable-game-versions"] = acceptableVersionsDeduped
			err = modpack.Write()
			if err != nil {
				fmt.Printf("Error writing pack: %s\n", err)
				os.Exit(1)
			}
			// Print success message
			prettyList := strings.Join(acceptableVersionsDeduped, ", ")
			prettyList += ", " + modpack.Versions["minecraft"]
			fmt.Printf("Set acceptable versions to %s\n", prettyList)
		}
	},
}

var flagAdd bool
var flagRemove bool

func init() {
	settingsCmd.AddCommand(acceptableVersionsCommand)

	// Add and remove flags for adding or removing specific versions
	acceptableVersionsCommand.Flags().BoolVarP(&flagAdd, "add", "a", false, "Add a version to the list")
	acceptableVersionsCommand.Flags().BoolVarP(&flagRemove, "remove", "r", false, "Remove a version from the list")
}
