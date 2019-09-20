package curseforge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/comp500/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type importPackFile interface {
	Name() string
	Open() (io.ReadCloser, error)
}

type importPackMetadata interface {
	Name() string
	Versions() map[string]string
	Mods() []struct {
		ModID  int
		FileID int
	}
	GetFiles() ([]importPackFile, error)
}

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import [modpack]",
	Short: "Import an installed curseforge modpack, from a download URL or a downloaded pack zip, or an installed metadata json file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]
		var packImport importPackMetadata

		if strings.HasPrefix(inputFile, "http") {
			fmt.Println("it do be a http doe")
			os.Exit(0)
		} else {
			// Attempt to read from file
			var f *os.File
			inputFileStat, err := os.Stat(inputFile)
			if err == nil && inputFileStat.IsDir() {
				// Apparently os.Open doesn't fail when file given is a directory, only when it gets read
				err = errors.New("cannot open directory")
			}
			if err == nil {
				f, err = os.Open(inputFile)
			}
			if err != nil {
				found := false
				var errInstance error
				var errManifest error
				var errCurse error

				// Look for other files/folders
				if _, errInstance = os.Stat(filepath.Join(inputFile, "minecraftinstance.json")); errInstance == nil {
					inputFile = filepath.Join(inputFile, "minecraftinstance.json")
					found = true
				} else if _, errManifest = os.Stat(filepath.Join(inputFile, "manifest.json")); errManifest == nil {
					inputFile = filepath.Join(inputFile, "manifest.json")
					found = true
				} else if runtime.GOOS == "windows" {
					var dir string
					dir, errCurse = getCurseDir()
					if errCurse == nil {
						curseInstanceFile := filepath.Join(dir, "Minecraft", "Instances", inputFile, "minecraftinstance.json")
						if _, errCurse = os.Stat(curseInstanceFile); errCurse == nil {
							inputFile = curseInstanceFile
							found = true
						}
					}
				}

				if found {
					f, err = os.Open(inputFile)
					if err != nil {
						fmt.Printf("Error opening file: %s\n", err)
						os.Exit(1)
					}
				} else {
					fmt.Printf("Error opening file: %s\n", err)
					fmt.Printf("Also attempted minecraftinstance.json: %s\n", errInstance)
					fmt.Printf("Also attempted manifest.json: %s\n", errManifest)
					if errCurse != nil {
						fmt.Printf("Also attempted to load a Curse/Twitch modpack named \"%s\": %s\n", inputFile, errCurse)
					}
					os.Exit(1)
				}
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
					packMeta.srcFile = inputFile
					packImport = packMeta
				}
			}
		}

		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println("Failed to load existing pack, creating a new one...")

			// Create a new modpack
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

			pack = core.Pack{
				Name: packImport.Name(),
				Index: struct {
					File       string `toml:"file"`
					HashFormat string `toml:"hash-format"`
					Hash       string `toml:"hash"`
				}{
					File: indexFilePath,
				},
				Versions: packImport.Versions(),
			}
		}
		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		modsList := packImport.Mods()
		modIDs := make([]int, len(modsList))
		for i, v := range modsList {
			modIDs[i] = v.ModID
		}

		fmt.Println("Querying Curse API for mod info...")

		modInfos, err := getModInfoMultiple(modIDs)
		if err != nil {
			fmt.Printf("Failed to obtain mod information: %s\n", err)
			os.Exit(1)
		}

		modInfosMap := make(map[int]modInfo)
		for _, v := range modInfos {
			modInfosMap[v.ID] = v
		}

		// TODO: multithreading????

		referencedModPaths := make([]string, 0, len(modsList))
		successes := 0
		for _, v := range modsList {
			modInfoValue, ok := modInfosMap[v.ModID]
			if !ok {
				fmt.Printf("Failed to obtain mod information for ID %d\n", v.ModID)
				continue
			}

			found := false
			var fileInfo modFileInfo
			for _, fileInfo = range modInfoValue.LatestFiles {
				if fileInfo.ID == v.FileID {
					found = true
					break
				}
			}
			if !found {
				fileInfo, err = getFileInfo(v.ModID, v.FileID)
				if err != nil {
					fmt.Printf("Failed to obtain file information for Mod / File %d / %d: %s\n", v.ModID, v.FileID, err)
					continue
				}
			}

			err = createModFile(modInfoValue, fileInfo, &index)
			if err != nil {
				fmt.Printf("Failed to save mod \"%s\": %s\n", modInfoValue.Name, err)
				os.Exit(1)
			}

			ref, err := filepath.Abs(filepath.Join(filepath.Dir(core.ResolveMod(modInfoValue.Slug)), fileInfo.FileName))
			if err == nil {
				referencedModPaths = append(referencedModPaths, ref)
				if len(ref) == 0 {
					fmt.Println(core.ResolveMod(modInfoValue.Slug))
					fmt.Println(filepath.Dir(core.ResolveMod(modInfoValue.Slug)))
				}
			}

			fmt.Printf("Imported mod \"%s\" successfully!\n", modInfoValue.Name)
			successes++
		}

		fmt.Printf("Successfully imported %d/%d mods!\n", successes, len(modsList))

		fmt.Println("Reading override files...")
		filesList, err := packImport.GetFiles()
		if err != nil {
			fmt.Printf("Failed to read override files: %s\n", err)
			os.Exit(1)
		}

		successes = 0
		indexFolder := filepath.Dir(filepath.Join(filepath.Dir(viper.GetString("pack-file")), filepath.FromSlash(pack.Index.File)))
		for _, v := range filesList {
			filePath := v.Name()
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(indexFolder, v.Name())
			}
			filePathAbs, err := filepath.Abs(filePath)
			if err == nil {
				found := false
				for _, v := range referencedModPaths {
					if v == filePathAbs {
						found = true
						break
					}
				}
				if found {
					fmt.Printf("Ignored file \"%s\" (referenced by metadata)\n", filePath)
					successes++
					continue
				}
				if filepath.Base(filePathAbs) == "minecraftinstance.json" {
					fmt.Println("Ignored file \"minecraftinstance.json\"")
					successes++
					continue
				}
				if filepath.Base(filePathAbs) == "manifest.json" {
					fmt.Println("Ignored file \"manifest.json\"")
					successes++
					continue
				}
			}

			f, err := os.Create(filePath)
			if err != nil {
				// Attempt to create the containing directory
				err2 := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
				if err2 == nil {
					f, err = os.Create(filePath)
				}
				if err != nil {
					fmt.Printf("Failed to write file \"%s\": %s\n", filePath, err)
					if err2 != nil {
						fmt.Printf("Failed to create directories: %s\n", err)
					}
					continue
				}
			}
			src, err := v.Open()
			if err != nil {
				fmt.Printf("Failed to read file \"%s\": %s\n", filePath, err)
				f.Close()
				continue
			}
			_, err = io.Copy(f, src)
			if err != nil {
				fmt.Printf("Failed to copy file \"%s\": %s\n", filePath, err)
				f.Close()
				src.Close()
				continue
			}

			fmt.Printf("Copied file \"%s\" successfully!\n", filePath)
			f.Close()
			src.Close()
			successes++
		}
		if len(filesList) > 0 {
			fmt.Printf("Successfully copied %d/%d files!\n", successes, len(filesList))
			err = index.Refresh()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			fmt.Println("No files copied!")
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
	},
}

func init() {
	curseforgeCmd.AddCommand(importCmd)
}

type diskFile struct {
	NameInternal string
	Path         string
}

func (f diskFile) Name() string {
	return f.NameInternal
}

func (f diskFile) Open() (io.ReadCloser, error) {
	return os.Open(f.Path)
}

func diskFilesFromPath(base string) ([]importPackFile, error) {
	list := make([]importPackFile, 0)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		list = append(list, diskFile{name, path})
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
		Name               string `json:"name"`
		MavenVersionString string `json:"mavenVersionString"`
	} `json:"baseModLoader"`
	ModpackOverrides []string `json:"modpackOverrides"`
	ModsInternal     []struct {
		ID   int `json:"addonID"`
		File struct {
			// I've given up on using this cached data, just going to re-request it
			ID int `json:"id"`
		} `json:"installedFile"`
	} `json:"installedAddons"`
	// Used to determine if modpackOverrides should be used or not
	IsUnlocked bool `json:"isUnlocked"`
	srcFile    string
}

func (m twitchInstalledPackMeta) Name() string {
	return m.NameInternal
}

func (m twitchInstalledPackMeta) Versions() map[string]string {
	vers := make(map[string]string)
	vers["minecraft"] = m.MCVersion
	if strings.HasPrefix(m.Modloader.Name, "forge") {
		if len(m.Modloader.MavenVersionString) > 0 {
			vers["forge"] = strings.TrimPrefix(m.Modloader.MavenVersionString, "net.minecraftforge:forge:")
		} else {
			vers["forge"] = m.MCVersion + "-" + strings.TrimPrefix(m.Modloader.Name, "forge-")
		}
	}
	return vers
}

func (m twitchInstalledPackMeta) Mods() []struct {
	ModID  int
	FileID int
} {
	list := make([]struct {
		ModID  int
		FileID int
	}, len(m.ModsInternal))
	for i, v := range m.ModsInternal {
		list[i] = struct {
			ModID  int
			FileID int
		}{
			ModID:  v.ID,
			FileID: v.File.ID,
		}
	}
	return list
}

func (m twitchInstalledPackMeta) GetFiles() ([]importPackFile, error) {
	dir := filepath.Dir(m.srcFile)
	if _, err := os.Stat(m.Path); err == nil {
		dir = m.Path
	}
	if m.IsUnlocked {
		return diskFilesFromPath(dir)
	}
	list := make([]importPackFile, len(m.ModpackOverrides))
	for i, v := range m.ModpackOverrides {
		list[i] = diskFile{
			Path:         filepath.Join(dir, v),
			NameInternal: v,
		}
	}
	return list, nil
}
