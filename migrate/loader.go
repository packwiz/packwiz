package migrate

import (
	"fmt"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"os"
)

var loaderCommand = &cobra.Command{
	Use:   "loader",
	Short: "Migrate your loader versions to newer versions.",
	Args:  cobra.ExactArgs(1),
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
		// Get our current loader
		currentLoader := modpack.GetLoaders()
		// Do some sanity checks on the current loader slice
		if len(currentLoader) == 0 {
			fmt.Println("No loader is currently set in your pack.toml!")
			os.Exit(1)
		} else if !slices.Contains(currentLoader, "quilt") && len(currentLoader) > 1 {
			fmt.Println("You have multiple loaders set in your pack.toml, this is not supported!")
			os.Exit(1)
		}
		// Get the Minecraft version for the pack
		mcVersion, err := modpack.GetMCVersion()
		if err != nil {
			fmt.Printf("Error getting Minecraft version: %s\n", err)
			os.Exit(1)
		}
		if args[0] == "latest" {
			fmt.Println("Updating to latest loader version")
			// We'll be updating to the latest loader version
			for _, loader := range currentLoader {
				loader, ok := core.ModLoaders[loader]
				if !ok {
					fmt.Printf("Unknown loader %s\n", loader)
					continue
				}
				_, latest, err := loader.VersionListGetter(mcVersion)
				if err != nil {
					fmt.Printf("Error getting version list for %s: %s\n", loader.Name, err)
					continue
				}
				// Check if the latest version is already set
				if latest == modpack.Versions[loader.Name] {
					fmt.Printf("Loader %s is already up to date!\n", loader.Name)
					continue
				}
				// Set the latest version
				modpack.Versions[loader.Name] = latest
				fmt.Printf("Updated loader %s to version %s\n", loader.Name, latest)
				// Write the pack to disk
				err = modpack.Write()
				if err != nil {
					fmt.Printf("Error writing pack.toml: %s\n", err)
					continue
				}
			}
		} else if args[0] == "recommended" {
			// TODO: Figure out a way to get the recommended version, this is Forge only
			fmt.Println("Currently updating to the recommended loader version is not supported!")
			os.Exit(1)
		} else {
			fmt.Println("Updating to explicit loader version")
			// Check if they're using quilt as we'll have 2 versions to update and will need to prompt for the versions
			if slices.Contains(currentLoader, "quilt") {
				// TODO: Prompt for the loader versions
			}
		}
	},
}

func init() {
	migrateCmd.AddCommand(loaderCommand)
}
