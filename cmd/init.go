package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fatih/camelcase"
	"github.com/igorsobreira/titlecase"
	"github.com/packwiz/packwiz/cmdshared"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise a packwiz modpack",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_, err := os.Stat(viper.GetString("pack-file"))
		if err == nil && !viper.GetBool("init.reinit") {
			fmt.Println("Modpack metadata file already exists, use -r to override!")
			os.Exit(1)
		} else if err != nil && !os.IsNotExist(err) {
			fmt.Printf("Error checking pack file: %s\n", err)
			os.Exit(1)
		}

		name, err := cmd.Flags().GetString("name")
		if err != nil || len(name) == 0 {
			// Get current file directory name
			wd, err := os.Getwd()
			directoryName := "."
			if err == nil {
				directoryName = filepath.Base(wd)
			}
			if directoryName != "." && len(directoryName) > 0 {
				// Turn directory name into a space-seperated proper name
				name = titlecase.Title(strings.ReplaceAll(strings.ReplaceAll(strings.Join(camelcase.Split(directoryName), " "), " - ", " "), " _ ", " "))
				name = initReadValue("Modpack name ["+name+"]: ", name)
			} else {
				name = initReadValue("Modpack name: ", "")
			}
		}

		author, err := cmd.Flags().GetString("author")
		if err != nil || len(author) == 0 {
			author = initReadValue("Author: ", "")
		}

		version, err := cmd.Flags().GetString("version")
		if err != nil || len(version) == 0 {
			version = initReadValue("Version [1.0.0]: ", "1.0.0")
		}

		mcVersions, err := cmdshared.GetValidMCVersions()
		if err != nil {
			fmt.Printf("Failed to get latest minecraft versions: %s\n", err)
			os.Exit(1)
		}

		mcVersion := viper.GetString("init.mc-version")
		if len(mcVersion) == 0 {
			var latestVersion string
			if viper.GetBool("init.snapshot") {
				latestVersion = mcVersions.Latest.Snapshot
			} else {
				latestVersion = mcVersions.Latest.Release
			}
			if viper.GetBool("init.latest") {
				mcVersion = latestVersion
			} else {
				mcVersion = initReadValue("Minecraft version ["+latestVersion+"]: ", latestVersion)
			}
		}
		mcVersions.CheckValid(mcVersion)

		modLoaderName := strings.ToLower(viper.GetString("init.modloader"))
		if len(modLoaderName) == 0 {
			modLoaderName = strings.ToLower(initReadValue("Mod loader [quilt]: ", "quilt"))
		}

		loader, ok := core.ModLoaders[modLoaderName]
		modLoaderVersions := make(map[string]string)
		if modLoaderName != "none" {
			if ok {
				versions, latestVersion, err := loader.VersionListGetter(mcVersion)
				if err != nil {
					fmt.Printf("Error loading versions: %s\n", err)
					os.Exit(1)
				}
				componentVersion := viper.GetString("init." + loader.Name + "-version")
				if len(componentVersion) == 0 {
					if viper.GetBool("init." + loader.Name + "-latest") {
						componentVersion = latestVersion
					} else {
						componentVersion = initReadValue(loader.FriendlyName+" version ["+latestVersion+"]: ", latestVersion)
					}
				}
				v := componentVersion
				// Forge uses a format where they prefix their version with their supported minecraft version. NeoForge
				// did this too, but only during the 1.20.1 days, they've since switched formats.
				if loader.Name == "forge" || (loader.Name == "neoforge" && mcVersion == "1.20.1") {
					v = cmdshared.GetRawForgeVersion(componentVersion)
				}
				if !slices.Contains(versions, v) {
					fmt.Println("Given " + loader.FriendlyName + " version cannot be found!")
					os.Exit(1)
				}
				modLoaderVersions[loader.Name] = v
			} else {
				fmt.Println("Given mod loader is not supported! Use \"none\" to specify no modloader, or to configure one manually.")
				fmt.Print("The following mod loaders are supported: ")
				keys := make([]string, len(core.ModLoaders))
				i := 0
				for k := range core.ModLoaders {
					keys[i] = k
					i++
				}
				fmt.Println(strings.Join(keys, ", "))
				os.Exit(1)
			}
		}

		indexFilePath := viper.GetString("init.index-file")
		_, err = os.Stat(indexFilePath)
		if os.IsNotExist(err) {
			// Create file
			err = os.WriteFile(indexFilePath, []byte{}, 0644)
			if err != nil {
				fmt.Printf("Error creating index file: %s\n", err)
				os.Exit(1)
			}
			fmt.Println(indexFilePath + " created!")
		} else if err != nil {
			fmt.Printf("Error checking index file: %s\n", err)
			os.Exit(1)
		}

		// Create the pack
		pack := core.Pack{
			Name:       name,
			Author:     author,
			Version:    version,
			PackFormat: core.CurrentPackFormat,
			Index: struct {
				File       string `toml:"file"`
				HashFormat string `toml:"hash-format"`
				Hash       string `toml:"hash,omitempty"`
			}{
				File: indexFilePath,
			},
			Versions: map[string]string{
				"minecraft": mcVersion,
			},
		}
		if modLoaderName != "none" {
			for k, v := range modLoaderVersions {
				pack.Versions[k] = v
			}
		}

		// Refresh the index and pack
		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = index.Refresh()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = index.Write()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = pack.Write()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(viper.GetString("pack-file") + " created!")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().String("name", "", "The name of the modpack (omit to define interactively)")
	initCmd.Flags().String("author", "", "The author of the modpack (omit to define interactively)")
	initCmd.Flags().String("version", "", "The version of the modpack (omit to define interactively)")
	initCmd.Flags().String("index-file", "index.toml", "The index file to use")
	_ = viper.BindPFlag("init.index-file", initCmd.Flags().Lookup("index-file"))
	initCmd.Flags().String("mc-version", "", "The Minecraft version to use (omit to define interactively)")
	_ = viper.BindPFlag("init.mc-version", initCmd.Flags().Lookup("mc-version"))
	initCmd.Flags().BoolP("latest", "l", false, "Automatically select the latest version of Minecraft")
	_ = viper.BindPFlag("init.latest", initCmd.Flags().Lookup("latest"))
	initCmd.Flags().BoolP("snapshot", "s", false, "Use the latest snapshot version with --latest")
	_ = viper.BindPFlag("init.snapshot", initCmd.Flags().Lookup("snapshot"))
	initCmd.Flags().BoolP("reinit", "r", false, "Recreate the pack file if it already exists, rather than exiting")
	_ = viper.BindPFlag("init.reinit", initCmd.Flags().Lookup("reinit"))
	initCmd.Flags().String("modloader", "", "The mod loader to use (omit to define interactively)")
	_ = viper.BindPFlag("init.modloader", initCmd.Flags().Lookup("modloader"))

	// ok this is epic
	for _, loader := range core.ModLoaders {
		initCmd.Flags().String(loader.Name+"-version", "", "The "+loader.FriendlyName+" version to use (omit to define interactively)")
		_ = viper.BindPFlag("init."+loader.Name+"-version", initCmd.Flags().Lookup(loader.Name+"-version"))
		initCmd.Flags().Bool(loader.Name+"-latest", false, "Automatically select the latest version of "+loader.FriendlyName)
		_ = viper.BindPFlag("init."+loader.Name+"-latest", initCmd.Flags().Lookup(loader.Name+"-latest"))
	}
}

func initReadValue(prompt string, def string) string {
	fmt.Print(prompt)
	if viper.GetBool("non-interactive") {
		fmt.Printf("%s\n", def)
		return def
	}
	value, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %s\n", err)
		os.Exit(1)
	}
	// Trims both CR and LF
	value = strings.TrimSpace(strings.TrimRight(value, "\r\n"))
	if len(value) > 0 {
		return value
	}
	return def
}
