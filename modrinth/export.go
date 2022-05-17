package modrinth

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
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

		// TODO: should index just expose indexPath itself, through a function?
		indexPath := filepath.Join(filepath.Dir(viper.GetString("pack-file")), filepath.FromSlash(pack.Index.File))

		mods, unwhitelistedMods := loadMods(index)

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

		// TODO: cache these (ideally with changes to pack format)
		fmt.Println("Retrieving hashes for external mods...")
		modsHashes := make([]map[string]string, len(mods))
		for i, mod := range mods {
			modsHashes[i], err = mod.GetHashes([]string{"sha1", "sha512", "length-bytes"})
			if err != nil {
				fmt.Printf("Error downloading mod file %s: %s\n", mod.Download.URL, err.Error())
				continue
			}
			fmt.Printf("Retrieved hashes for %s successfully\n", mod.Download.URL)
		}

		manifestFile, err := exp.Create("modrinth.index.json")
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error creating manifest: " + err.Error())
			os.Exit(1)
		}

		manifestFiles := make([]PackFile, len(mods))
		for i, mod := range mods {
			pathForward, err := filepath.Rel(filepath.Dir(indexPath), mod.GetDestFilePath())
			if err != nil {
				fmt.Printf("Error resolving mod file: %s\n", err.Error())
				// TODO: exit(1)?
				continue
			}

			path := filepath.ToSlash(pathForward)

			hashes := make(map[string]string)
			hashes["sha1"] = modsHashes[i]["sha1"]
			hashes["sha512"] = modsHashes[i]["sha512"]
			fileSize, err := strconv.ParseUint(modsHashes[i]["length-bytes"], 10, 64)
			if err != nil {
				panic(err)
			}

			// Create env options based on configured optional/side
			var envInstalled string
			if mod.Option != nil && mod.Option.Optional {
				envInstalled = "optional"
			} else {
				envInstalled = "required"
			}
			var clientEnv, serverEnv string
			if mod.Side == core.UniversalSide {
				clientEnv = envInstalled
				serverEnv = envInstalled
			} else if mod.Side == core.ClientSide {
				clientEnv = envInstalled
				serverEnv = "unsupported"
			} else if mod.Side == core.ServerSide {
				clientEnv = "unsupported"
				serverEnv = envInstalled
			}

			// Modrinth URLs must be RFC3986
			u, err := core.ReencodeURL(mod.Download.URL)
			if err != nil {
				fmt.Printf("Error re-encoding mod URL: %s\n", err.Error())
				u = mod.Download.URL
			}

			manifestFiles[i] = PackFile{
				Path:   path,
				Hashes: hashes,
				Env: &struct {
					Client string `json:"client"`
					Server string `json:"server"`
				}{Client: clientEnv, Server: serverEnv},
				Downloads: []string{u},
				FileSize:  uint32(fileSize),
			}
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

		w := json.NewEncoder(manifestFile)
		w.SetIndent("", "    ") // Documentation uses 4 spaces
		err = w.Encode(manifest)
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error writing manifest: " + err.Error())
			os.Exit(1)
		}

		for _, v := range index.Files {
			if !v.MetaFile {
				// Save all non-metadata files into the zip
				path, err := filepath.Rel(filepath.Dir(indexPath), index.GetFilePath(v))
				if err != nil {
					fmt.Printf("Error resolving file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				file, err := exp.Create(filepath.ToSlash(filepath.Join("overrides", path)))
				if err != nil {
					fmt.Printf("Error creating file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				err = index.SaveFile(v, file)
				if err != nil {
					fmt.Printf("Error copying file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
			}
		}

		if len(unwhitelistedMods) > 0 {
			fmt.Println("Downloading unwhitelisted mods...")
		}
		for _, v := range unwhitelistedMods {
			pathRel, err := filepath.Rel(filepath.Dir(indexPath), v.GetDestFilePath())
			if err != nil {
				fmt.Printf("Error resolving mod file: %s\n", err.Error())
				// TODO: exit(1)?
				continue
			}
			var path string
			if v.Side == core.ClientSide {
				path = filepath.ToSlash(filepath.Join("client-overrides", pathRel))
			} else if v.Side == core.ServerSide {
				path = filepath.ToSlash(filepath.Join("server-overrides", pathRel))
			} else {
				path = filepath.ToSlash(filepath.Join("overrides", pathRel))
			}

			file, err := exp.Create(path)
			if err != nil {
				fmt.Printf("Error creating file: %s\n", err.Error())
				// TODO: exit(1)?
				continue
			}
			err = v.DownloadFile(file)
			if err != nil {
				fmt.Printf("Error downloading file: %s\n", err.Error())
				// TODO: exit(1)?
				continue
			}
			fmt.Printf("Downloaded %v successfully\n", path)
		}

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
	"edge.forgecdn.net",
	"github.com",
	"raw.githubusercontent.com",
}

func loadMods(index core.Index) ([]core.Mod, []core.Mod) {
	modPaths := index.GetAllMods()
	mods := make([]core.Mod, 0, len(modPaths))
	unwhitelistedMods := make([]core.Mod, 0)
	fmt.Println("Reading mod files...")
	for _, v := range modPaths {
		modData, err := core.LoadMod(v)
		if err != nil {
			fmt.Printf("Error reading mod file %s: %s\n", v, err.Error())
			// TODO: exit(1)?
			continue
		}

		modUrl, err := url.Parse(modData.Download.URL)
		if err == nil {
			if slices.Contains(whitelistedHosts, modUrl.Host) {
				mods = append(mods, modData)
			} else {
				unwhitelistedMods = append(unwhitelistedMods, modData)
			}
		} else {
			fmt.Printf("Failed to parse mod URL: %v\n", modUrl)
			mods = append(mods, modData)
		}
	}
	return mods, unwhitelistedMods
}

func init() {
	modrinthCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringP("output", "o", "", "The file to export the modpack to")
	_ = viper.BindPFlag("modrinth.export.output", exportCmd.Flags().Lookup("output"))
}
