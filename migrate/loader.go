package migrate

import (
	"fmt"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"os"
	"strings"
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
		} else if slices.Contains(currentLoader, "quilt") {
			// We have quilt, so we need to remove fabric from the loaders list
			fabricIndex := slices.Index(currentLoader, "fabric")
			currentLoader = slices.Delete(currentLoader, fabricIndex, fabricIndex+1)
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
				if !updatePackToVersion(latest, modpack, gottenLoader) {
					continue
				}
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
			// This one is easy :D
			versions, _, loader := getVersionsForLoader(currentLoader[0], mcVersion)
			// Check if the loader happens to be Forge, since there's two version formats
			if loader.Name == "forge" {
				var wantedVersion string
				// Check if we have a "-" in the version
				if strings.Contains(args[0], "-") {
					// We have a mcVersion-loaderVersion format
					// Strip the mcVersion
					wantedVersion = strings.Split(args[0], "-")[1]
				} else {
					wantedVersion = args[0]
				}
				validateVersion(versions, wantedVersion, loader)
				_ = updatePackToVersion(wantedVersion, modpack, loader)
			} else if loader.Name == "liteloader" {
				// These are weird and just have a MC version
				fmt.Println("LiteLoader only has 1 version per Minecraft version so we're unable to update!")
				os.Exit(0)
			} else {
				// We're on Fabric or quilt
				validateVersion(versions, args[0], loader)
				_ = updatePackToVersion(args[0], modpack, loader)
			}
			// Write the pack to disk
			err = modpack.Write()
			if err != nil {
				fmt.Printf("Error writing pack.toml: %s\n", err)
				os.Exit(1)
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
		fmt.Printf("Error getting version list for %s: %s\n", gottenLoader.FriendlyName, err)
		os.Exit(1)
	}
	return versions, latestVersion, gottenLoader
}

func validateVersion(versions []string, version string, gottenLoader core.ModLoaderComponent) {
	if !slices.Contains(versions, version) {
		fmt.Printf("Version %s is not a valid version for %s\n", version, gottenLoader.FriendlyName)
		os.Exit(1)
	}
}

func updatePackToVersion(version string, modpack core.Pack, loader core.ModLoaderComponent) bool {
	// Check if the version is already set
	if version == modpack.Versions[loader.Name] {
		fmt.Printf("%s is already on version %s!\n", loader.FriendlyName, version)
		return false
	}
	// Set the latest version
	modpack.Versions[loader.Name] = version
	fmt.Printf("Updated %s to version %s\n", loader.FriendlyName, version)
	return true
}
