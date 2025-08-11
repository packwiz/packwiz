package modrinth

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"

	"github.com/packwiz/packwiz/cmdshared"
	"github.com/spf13/viper"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the current modpack into a .mrpack for Modrinth",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		// Do a refresh to ensure files are up to date
		err = index.Refresh()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = index.Write()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = pack.Write()
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("Reading external files...")
		mods, err := index.LoadAllMods()
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			os.Exit(1)
		}

		fileName := viper.GetString("modrinth.export.output")
		if fileName == "" {
			fileName = pack.GetPackName() + ".mrpack"
		}
		expFile, err := os.Create(fileName)
		if err != nil {
			fmt.Printf("Failed to create zip: %s\n", err.Error())
			os.Exit(1)
		}
		exp := zip.NewWriter(expFile)

		// Add an overrides folder even if there are no files to go in it
		_, err = exp.Create("overrides/")
		if err != nil {
			fmt.Printf("Failed to add overrides folder: %s\n", err.Error())
			os.Exit(1)
		}

		fmt.Printf("Retrieving %v external files...\n", len(mods))

		restrictDomains := viper.GetBool("modrinth.export.restrictDomains")

		for _, mod := range mods {
			if !canBeIncludedDirectly(mod, restrictDomains) {
				cmdshared.PrintDisclaimer(false)
				break
			}
		}

		session, err := core.CreateDownloadSession(mods, []string{"sha1", "sha512", "length-bytes"})
		if err != nil {
			fmt.Printf("Error retrieving external files: %v\n", err)
			os.Exit(1)
		}

		cmdshared.ListManualDownloads(session)

		manifestFiles := make([]PackFile, 0)
		for dl := range session.StartDownloads() {
			if canBeIncludedDirectly(dl.Mod, restrictDomains) {
				if dl.Error != nil {
					fmt.Printf("Download of %s (%s) failed: %v\n", dl.Mod.Name, dl.Mod.FileName, dl.Error)
					continue
				}
				for _, warning := range dl.Warnings {
					fmt.Printf("Warning for %s (%s): %v\n", dl.Mod.Name, dl.Mod.FileName, warning)
				}

				path, err := index.RelIndexPath(dl.Mod.GetDestFilePath())
				if err != nil {
					fmt.Printf("Error resolving external file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}

				hashes := make(map[string]string)
				hashes["sha1"] = dl.Hashes["sha1"]
				hashes["sha512"] = dl.Hashes["sha512"]
				fileSize, err := strconv.ParseUint(dl.Hashes["length-bytes"], 10, 64)
				if err != nil {
					panic(err)
				}

				// Create env options based on configured optional/side
				var envInstalled string
				if dl.Mod.Option != nil && dl.Mod.Option.Optional {
					envInstalled = "optional"
				} else {
					envInstalled = "required"
				}
				var clientEnv, serverEnv string
				if dl.Mod.Side == core.UniversalSide || dl.Mod.Side == core.EmptySide {
					clientEnv = envInstalled
					serverEnv = envInstalled
				} else if dl.Mod.Side == core.ClientSide {
					clientEnv = envInstalled
					serverEnv = "unsupported"
				} else if dl.Mod.Side == core.ServerSide {
					clientEnv = "unsupported"
					serverEnv = envInstalled
				}

				// Modrinth URLs must be RFC3986
				u, err := core.ReencodeURL(dl.Mod.Download.URL)
				if err != nil {
					fmt.Printf("Error re-encoding download URL: %s\n", err.Error())
					u = dl.Mod.Download.URL
				}

				manifestFiles = append(manifestFiles, PackFile{
					Path:   path,
					Hashes: hashes,
					Env: &struct {
						Client string `json:"client"`
						Server string `json:"server"`
					}{Client: clientEnv, Server: serverEnv},
					Downloads: []string{u},
					FileSize:  uint32(fileSize),
				})

				fmt.Printf("%s (%s) added to manifest\n", dl.Mod.Name, dl.Mod.FileName)
			} else {
				if dl.Mod.Side == core.ClientSide {
					_ = cmdshared.AddToZip(dl, exp, "client-overrides", &index)
				} else if dl.Mod.Side == core.ServerSide {
					_ = cmdshared.AddToZip(dl, exp, "server-overrides", &index)
				} else {
					_ = cmdshared.AddToZip(dl, exp, "overrides", &index)
				}
			}
		}
		// sort by `path` property before serialising to ensure reproducibility
		sort.Slice(manifestFiles, func(i, j int) bool {
			return manifestFiles[i].Path < manifestFiles[j].Path
		})

		err = session.SaveIndex()
		if err != nil {
			fmt.Printf("Error saving cache index: %v\n", err)
			os.Exit(1)
		}

		dependencies := make(map[string]string)
		dependencies["minecraft"], err = pack.GetMCVersion()
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error creating manifest: " + err.Error())
			os.Exit(1)
		}
		if quiltVersion, ok := pack.Versions["quilt"]; ok {
			dependencies["quilt-loader"] = quiltVersion
		} else if fabricVersion, ok := pack.Versions["fabric"]; ok {
			dependencies["fabric-loader"] = fabricVersion
		} else if forgeVersion, ok := pack.Versions["forge"]; ok {
			dependencies["forge"] = forgeVersion
		} else if neoforgeVersion, ok := pack.Versions["neoforge"]; ok {
			dependencies["neoforge"] = neoforgeVersion
		}

		manifest := Pack{
			FormatVersion: 1,
			Game:          "minecraft",
			VersionID:     pack.Version,
			Name:          pack.Name,
			Summary:       pack.Description,
			Files:         manifestFiles,
			Dependencies:  dependencies,
		}

		if len(pack.Version) == 0 {
			fmt.Println("Warning: pack.toml version field must not be empty to create a valid Modrinth pack")
		}

		manifestFile, err := exp.Create("modrinth.index.json")
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error creating manifest: " + err.Error())
			os.Exit(1)
		}

		w := json.NewEncoder(manifestFile)
		w.SetIndent("", "    ") // Documentation uses 4 spaces
		err = w.Encode(manifest)
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error writing manifest: " + err.Error())
			os.Exit(1)
		}

		cmdshared.AddNonMetafileOverrides(&index, exp)

		err = exp.Close()
		if err != nil {
			fmt.Println("Error writing export file: " + err.Error())
			os.Exit(1)
		}
		err = expFile.Close()
		if err != nil {
			fmt.Println("Error writing export file: " + err.Error())
			os.Exit(1)
		}

		fmt.Println("Modpack exported to " + fileName)
	},
}

var whitelistedHosts = []string{
	"cdn.modrinth.com",
	"github.com",
	"raw.githubusercontent.com",
	"gitlab.com",
}

func canBeIncludedDirectly(mod *core.Mod, restrictDomains bool) bool {
	if mod.Download.Mode == core.ModeURL || mod.Download.Mode == "" {
		if !restrictDomains {
			return true
		}

		modUrl, err := url.Parse(mod.Download.URL)
		if err == nil {
			if slices.Contains(whitelistedHosts, modUrl.Host) {
				return true
			}
		}
	}
	return false
}

func init() {
	modrinthCmd.AddCommand(exportCmd)
	exportCmd.Flags().Bool("restrictDomains", true, "Restricts domains to those allowed by modrinth.com")
	exportCmd.Flags().StringP("output", "o", "", "The file to export the modpack to")
	_ = viper.BindPFlag("modrinth.export.restrictDomains", exportCmd.Flags().Lookup("restrictDomains"))
	_ = viper.BindPFlag("modrinth.export.output", exportCmd.Flags().Lookup("output"))
}
