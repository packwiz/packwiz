package cmd

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/comp500/packwiz/core"
	"github.com/fatih/camelcase"
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
				name = strings.ReplaceAll(strings.ReplaceAll(strings.Join(camelcase.Split(directoryName), " "), " - ", " "), " _ ", " ")
				fmt.Print("Modpack name [" + name + "]: ")
			} else {
				fmt.Print("Modpack name: ")
			}
			readName, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading input: %s\n", err)
				os.Exit(1)
			}
			// Trims both CR and LF
			readName = strings.TrimSpace(strings.TrimRight(readName, "\r\n"))
			if len(readName) > 0 {
				name = readName
			}
		}

		mcVersions, err := getValidMCVersions()
		if err != nil {
			fmt.Printf("Failed to get latest minecraft versions: %s", err)
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
				fmt.Print("Minecraft version [" + latestVersion + "]: ")
				mcVersion, err = bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					fmt.Printf("Error reading input: %s\n", err)
					os.Exit(1)
				}
				// Trims both CR and LF
				mcVersion = strings.TrimSpace(strings.TrimRight(mcVersion, "\r\n"))
				if len(mcVersion) == 0 {
					mcVersion = latestVersion
				}
			}
		}
		mcVersions.checkValid(mcVersion)

		// TODO: minecraft modloader
		modLoaderName := viper.GetString("init.modloader")
		if len(modLoaderName) == 0 {
			var defaultLoader string
			if viper.GetBool("init.snapshot") {
				defaultLoader = "fabric"
			} else {
				defaultLoader = "forge"
			}
			fmt.Print("Mod loader [" + defaultLoader + "]: ")
			modLoaderName, err = bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading input: %s\n", err)
				os.Exit(1)
			}
			// Trims both CR and LF
			modLoaderName = strings.ToLower(strings.TrimSpace(strings.TrimRight(modLoaderName, "\r\n")))
			if len(modLoaderName) == 0 {
				modLoaderName = defaultLoader
			}
		}
		_, ok := modLoaders[modLoaderName]
		if modLoaderName != "none" && !ok {
			fmt.Println("Given mod loader is not supported! Use \"none\" to specify no modloader, or to configure one manually.")
			fmt.Print("The following mod loaders are supported: ")
			keys := make([]string, len(modLoaders))
			i := 0
			for k := range modLoaders {
				keys[i] = k
				i++
			}
			fmt.Println(strings.Join(keys, ", "))
			os.Exit(1)
		}

		var modLoaderVersion string
		if modLoaderName != "none" {
			versions, latestVersion, err := modLoaders[modLoaderName](mcVersion)
			modLoaderVersion = viper.GetString("init.modloader-version")
			if len(modLoaderVersion) == 0 {
				if viper.GetBool("init.modloader-latest") {
					modLoaderVersion = latestVersion
				} else {
					fmt.Print("Mod loader version [" + latestVersion + "]: ")
					modLoaderVersion, err = bufio.NewReader(os.Stdin).ReadString('\n')
					if err != nil {
						fmt.Printf("Error reading input: %s\n", err)
						os.Exit(1)
					}
					// Trims both CR and LF
					modLoaderVersion = strings.ToLower(strings.TrimSpace(strings.TrimRight(modLoaderVersion, "\r\n")))
					if len(modLoaderVersion) == 0 {
						modLoaderVersion = latestVersion
					}
				}
			}
			found := false
			for _, v := range versions {
				if modLoaderVersion == v {
					found = true
					break
				}
			}
			if !found {
				fmt.Println("Given mod loader version cannot be found!")
				os.Exit(1)
			}
		}

		indexFilePath := viper.GetString("init.index-file")
		_, err = os.Stat(indexFilePath)
		if os.IsNotExist(err) {
			// Create file
			err = ioutil.WriteFile(indexFilePath, []byte{}, 0644)
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
			Name: name,
			Index: struct {
				File       string `toml:"file"`
				HashFormat string `toml:"hash-format"`
				Hash       string `toml:"hash"`
			}{
				File: indexFilePath,
			},
			Versions: map[string]string{
				"minecraft": mcVersion,
			},
		}
		if modLoaderName != "none" {
			pack.Versions[modLoaderName] = modLoaderVersion
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
	initCmd.Flags().String("index-file", "index.toml", "The index file to use")
	viper.BindPFlag("init.index-file", initCmd.Flags().Lookup("index-file"))
	initCmd.Flags().String("mc-version", "", "The version of Minecraft to use (omit to define interactively)")
	viper.BindPFlag("init.mc-version", initCmd.Flags().Lookup("mc-version"))
	initCmd.Flags().BoolP("latest", "l", false, "Automatically select the latest version of Minecraft")
	viper.BindPFlag("init.latest", initCmd.Flags().Lookup("latest"))
	initCmd.Flags().BoolP("snapshot", "s", false, "Use the latest snapshot version with --latest")
	viper.BindPFlag("init.snapshot", initCmd.Flags().Lookup("snapshot"))
	initCmd.Flags().BoolP("reinit", "r", false, "Recreate the pack file if it already exists, rather than exiting")
	viper.BindPFlag("init.reinit", initCmd.Flags().Lookup("reinit"))
	initCmd.Flags().String("modloader", "", "The mod loader to use (omit to define interactively)")
	viper.BindPFlag("init.modloader", initCmd.Flags().Lookup("modloader"))
	initCmd.Flags().String("modloader-version", "", "The mod loader version to use (omit to define interactively)")
	viper.BindPFlag("init.modloader-version", initCmd.Flags().Lookup("modloader-version"))
	initCmd.Flags().BoolP("modloader-latest", "L", false, "Automatically select the latest version of the mod loader")
	viper.BindPFlag("init.modloader-latest", initCmd.Flags().Lookup("modloader-latest"))
}

type mcVersionManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []struct {
		ID          string    `json:"id"`
		Type        string    `json:"type"`
		URL         string    `json:"url"`
		Time        time.Time `json:"time"`
		ReleaseTime time.Time `json:"releaseTime"`
	} `json:"versions"`
}

func (m mcVersionManifest) checkValid(version string) {
	for _, v := range m.Versions {
		if v.ID == version {
			return
		}
	}
	fmt.Println("Given version is not a valid Minecraft version!")
	os.Exit(1)
}

func getValidMCVersions() (mcVersionManifest, error) {
	res, err := http.Get("https://launchermeta.mojang.com/mc/game/version_manifest.json")
	if err != nil {
		return mcVersionManifest{}, err
	}
	dec := json.NewDecoder(res.Body)
	out := mcVersionManifest{}
	err = dec.Decode(&out)
	if err != nil {
		return mcVersionManifest{}, err
	}
	// Sort by newest to oldest
	sort.Slice(out.Versions, func(i, j int) bool {
		return out.Versions[i].ReleaseTime.Before(out.Versions[j].ReleaseTime)
	})
	return out, nil
}

type mavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Versioning struct {
		Release  string `xml:"release"`
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
		LastUpdated string `xml:"lastUpdated"`
	} `xml:"versioning"`
}

// Gets a list of modloader versions and latest version for a given Minecraft version
var modLoaders = map[string]func(mcVersion string) ([]string, string, error){
	"fabric": func(mcVersion string) ([]string, string, error) {
		res, err := http.Get("https://maven.fabricmc.net/net/fabricmc/fabric-loader/maven-metadata.xml")
		if err != nil {
			return []string{}, "", err
		}
		dec := xml.NewDecoder(res.Body)
		out := mavenMetadata{}
		err = dec.Decode(&out)
		if err != nil {
			return []string{}, "", err
		}
		return out.Versioning.Versions.Version, out.Versioning.Release, nil
	},
	"forge": func(mcVersion string) ([]string, string, error) {
		res, err := http.Get("https://files.minecraftforge.net/maven/net/minecraftforge/forge/maven-metadata.xml")
		if err != nil {
			return []string{}, "", err
		}
		dec := xml.NewDecoder(res.Body)
		out := mavenMetadata{}
		err = dec.Decode(&out)
		if err != nil {
			return []string{}, "", err
		}
		allowedVersions := make([]string, 0, len(out.Versioning.Versions.Version))
		for _, v := range out.Versioning.Versions.Version {
			if strings.HasPrefix(v, mcVersion) {
				allowedVersions = append(allowedVersions, v)
			}
		}
		if len(allowedVersions) == 0 {
			return []string{}, "", errors.New("no Forge versions available for this Minecraft version")
		}
		if strings.HasPrefix(out.Versioning.Release, mcVersion) {
			return allowedVersions, out.Versioning.Release, nil
		}
		return allowedVersions, allowedVersions[len(allowedVersions)-1], nil
	},
}
