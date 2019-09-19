package curseforge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/comp500/packwiz/core"
	"github.com/spf13/cobra"
)

type importPackFile interface {
	Name() string
	Open() (io.ReadCloser, error)
}

type importPackMetadata interface {
	Name() string
	Versions() map[string]string
	Mods() []struct {
		ID   int
		File modFileInfo
	}
	GetFiles() ([]importPackFile, error)
}

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import [modpack]",
	Short: "Import an installed curseforge modpack, from a download URL or a downloaded pack zip, or an installed metadata json file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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

		var packImport importPackMetadata
		if strings.HasPrefix(args[0], "http") {
			fmt.Println("it do be a http doe")
			os.Exit(0)
		} else {
			// Attempt to read from file
			f, err := os.Open(args[0])
			if err != nil {
				fmt.Printf("Error opening file: %s\n", err)
				os.Exit(1)
			}
			defer f.Close()

			buf := bufio.NewReader(f)
			header, err := buf.Peek(2)
			if err != nil {
				fmt.Printf("Error reading file: %s\n", err)
				os.Exit(1)
			}

			// Check if file is a zip
			if string(header) == "PK" {
				fmt.Println("it do be a zip doe")
				os.Exit(0)
			} else {
				// Read the whole file (as we are going to parse it multiple times)
				fileData, err := ioutil.ReadAll(buf)
				if err != nil {
					fmt.Printf("Error reading file: %s\n", err)
					os.Exit(1)
				}

				// Determine what format the file is
				var jsonFile map[string]interface{}
				err = json.Unmarshal(fileData, &jsonFile)
				if err != nil {
					fmt.Printf("Error parsing JSON: %s\n", err)
					os.Exit(1)
				}

				isManifest := false
				if v, ok := jsonFile["manifestType"]; ok {
					isManifest = v.(string) == "minecraftModpack"
				}
				if isManifest {
					fmt.Println("it do be a manifest doe")
					os.Exit(0)
				} else {
					// Replace FileNameOnDisk with fileNameOnDisk
					fileData = bytes.ReplaceAll(fileData, []byte("FileNameOnDisk"), []byte("fileNameOnDisk"))
					packMeta := twitchInstalledPackMeta{}
					err = json.Unmarshal(fileData, &packMeta)
					if err != nil {
						fmt.Printf("Error parsing JSON: %s\n", err)
						os.Exit(1)
					}
					packImport = packMeta
				}
			}
		}

		modsList := packImport.Mods()
		modIDs := make([]int, len(modsList))
		for i, v := range modsList {
			modIDs[i] = v.ID
		}

		fmt.Println("Querying Curse API for mod info...")

		modInfos, err := getModInfoMultiple(modIDs)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		modInfosMap := make(map[int]modInfo)
		for _, v := range modInfos {
			modInfosMap[v.ID] = v
		}

		// TODO: multithreading????
		for _, v := range modsList {
			modInfoValue, ok := modInfosMap[v.ID]
			if !ok {
				if len(v.File.FriendlyName) > 0 {
					fmt.Printf("Failed to obtain mod information for \"%s\"\n", v.File.FriendlyName)
				} else {
					fmt.Printf("Failed to obtain mod information for \"%s\"\n", v.File.FileName)
				}
				continue
			}

			fmt.Printf("Imported mod \"%s\" successfully!\n", modInfoValue.Name)

			err = createModFile(modInfoValue, v.File, &index)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		// TODO: import existing files (config etc.)

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
	},
}

func init() {
	curseforgeCmd.AddCommand(importCmd)
}

type diskFile struct {
	NameInternal string
	Base         string
}

func (f diskFile) Name() string {
	return f.NameInternal
}

func (f diskFile) Open() (io.ReadCloser, error) {
	return os.Open(filepath.Join(f.Base, f.NameInternal))
}

func diskFilesFromPath(base string) ([]importPackFile, error) {
	list := make([]importPackFile, 0)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		list = append(list, diskFile{base, path})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

type twitchInstalledPackMeta struct {
	NameInternal string `json:"name"`
	Path         string `json:"installPath"`
	// TODO: javaArgsOverride?
	// TODO: allocatedMemory?
	MCVersion string `json:"gameVersion"`
	Modloader struct {
		name               string
		mavenVersionString string
	} `json:"baseModLoader"`
	ModpackOverrides []string `json:"modpackOverrides"`
	ModsInternal     []struct {
		ID   int         `json:"addonID"`
		File modFileInfo `json:"installedFile"`
	} `json:"installedAddons"`
	// Used to determine if modpackOverrides should be used or not
	IsUnlocked bool `json:"isUnlocked"`
}

func (m twitchInstalledPackMeta) Name() string {
	return m.NameInternal
}

func (m twitchInstalledPackMeta) Versions() map[string]string {
	vers := make(map[string]string)
	vers["minecraft"] = m.MCVersion
	if strings.HasPrefix(m.Modloader.name, "forge") {
		if len(m.Modloader.mavenVersionString) > 0 {
			vers["forge"] = strings.TrimPrefix(m.Modloader.mavenVersionString, "net.minecraftforge:forge:")
		} else {
			vers["forge"] = m.MCVersion + "-" + strings.TrimPrefix(m.Modloader.name, "forge-")
		}
	}
	return vers
}

func (m twitchInstalledPackMeta) Mods() []struct {
	ID   int
	File modFileInfo
} {
	return []struct {
		ID   int
		File modFileInfo
	}(m.ModsInternal)
}

func (m twitchInstalledPackMeta) GetFiles() ([]importPackFile, error) {
	if m.IsUnlocked {
		return diskFilesFromPath(m.Path)
	}
	list := make([]importPackFile, len(m.ModpackOverrides))
	for i, v := range m.ModpackOverrides {
		list[i] = diskFile{
			Base:         m.Path,
			NameInternal: v,
		}
	}
	return list, nil
}
