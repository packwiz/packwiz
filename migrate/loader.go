package migrate

import (
	"fmt"
	"os"
	"slices"

	"github.com/packwiz/packwiz/cmdshared"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

var loaderCommand = &cobra.Command{
	Use:   "loader [version|latest|recommended]",
	Short: "Migrate your modloader version to a newer version.",
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
		var currentLoaders = modpack.GetLoaders()
		// Do some sanity checks on the current loader slice
		if len(currentLoaders) == 0 {
			fmt.Println("No loader is currently set in your pack.toml!")
			os.Exit(1)
		} else if len(currentLoaders) > 1 {
			fmt.Println("You have multiple loaders set in your pack.toml, this is not supported!")
			os.Exit(1)
		}
		// Get the Minecraft version for the pack
		mcVersion, err := modpack.GetMCVersion()
		if err != nil {
			fmt.Printf("Error getting Minecraft version: %s\n", err)
			os.Exit(1)
		}
		if args[0] == "latest" || args[0] == "recommended" {
			fmt.Printf("Updating to %s loader version\n", args[0])

			queryType := core.Latest
			if args[0] == "recommended" {
				queryType = core.Recommended
			}

			// We'll be updating to the latest loader version
			for _, loader := range currentLoaders {
				versionData, gottenLoader := getVersionsForLoader(loader, mcVersion, queryType)
				if !updatePackToVersion(versionData.Latest, modpack, gottenLoader) {
					continue
				}
				// Write the pack to disk
				err = modpack.Write()
				if err != nil {
					fmt.Printf("Error writing pack.toml: %s\n", err)
					continue
				}
			}
		} else {
			fmt.Println("Updating to explicit loader version")
			// This one is easy :D
			versionData, loader := getVersionsForLoader(currentLoaders[0], mcVersion, core.Latest)
			// Check if the loader happens to be Forge/NeoForge, since there's two version formats
			if loader.Name == "forge" || loader.Name == "neoforge" {
				wantedVersion := cmdshared.GetRawForgeVersion(args[0])
				validateVersion(versionData.Versions, wantedVersion, loader)
				_ = updatePackToVersion(wantedVersion, modpack, loader)
			} else if loader.Name == "liteloader" {
				// These are weird and just have a MC version
				fmt.Println("LiteLoader only has 1 version per Minecraft version so we're unable to update!")
				os.Exit(0)
			} else {
				// We're on Fabric or quilt
				validateVersion(versionData.Versions, args[0], loader)
				if ok := updatePackToVersion(args[0], modpack, loader); !ok {
					os.Exit(1)
				}
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

func getVersionsForLoader(loader, mcVersion string, queryType core.QueryType) (*core.ModLoaderVersions, core.ModLoaderComponent) {
	gottenLoader, ok := core.ModLoaders[loader]
	if !ok {
		fmt.Printf("Unknown loader %s\n", loader)
		os.Exit(1)
	}
	versionData, err := core.DoQuery(core.MakeQuery(gottenLoader, mcVersion).WithQueryType(queryType))
	if err != nil {
		fmt.Printf("Error getting version list for %s: %s\n", gottenLoader.FriendlyName, err)
		os.Exit(1)
	}
	return versionData, gottenLoader
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
