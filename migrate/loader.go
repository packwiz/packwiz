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
				_, latest, gottenLoader := getVersionsForLoader(loader, mcVersion)
				// Check if the latest version is already set
				if latest == modpack.Versions[gottenLoader.Name] {
					fmt.Printf("Loader %s is already up to date!\n", gottenLoader.Name)
					continue
				}
				// Set the latest version
				modpack.Versions[gottenLoader.Name] = latest
				fmt.Printf("Updated loader %s to version %s\n", gottenLoader.Name, latest)
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
			} else {
				// This one is easy :D
				versions, _, loader := getVersionsForLoader(currentLoader[0], mcVersion)
				// Check if the loader happens to be Forge, since there's two version formats
				if loader.Name == "forge" {
					// TODO: Handle both mcVersion-loaderVersion and loaderVersion
				} else if loader.Name == "liteloader" {
					// These are weird and just have a MC version
					fmt.Println("LiteLoader only has 1 version per Minecraft version so we're unable to update!")
					os.Exit(0)
				} else {
					// We're on Fabric
					// Check if the given version is in the list
					if !slices.Contains(versions, args[0]) {
						fmt.Printf("Version %s is not a valid version for %s\n", args[0], loader.Name)
						os.Exit(1)
					}
					// Set the version
					modpack.Versions[loader.Name] = args[0]
					fmt.Printf("Updated loader %s to version %s\n", loader.Name, args[0])
					// Write the pack to disk
					err = modpack.Write()
					if err != nil {
						fmt.Printf("Error writing pack.toml: %s\n", err)
						os.Exit(1)
					}
				}
			}
		}
	},
}

func init() {
	migrateCmd.AddCommand(loaderCommand)
}

func getVersionsForLoader(loader, mcVersion string) ([]string, string, core.ModLoaderComponent) {
	gottenLoader, ok := core.ModLoaders[loader]
	if !ok {
		fmt.Printf("Unknown loader %s\n", loader)
		os.Exit(1)
	}
	versions, latestVersion, err := gottenLoader.VersionListGetter(mcVersion)
	if err != nil {
		fmt.Printf("Error getting version list for %s: %s\n", gottenLoader.Name, err)
		os.Exit(1)
	}
	return versions, latestVersion, gottenLoader
}
