package curseforge

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/comp500/packwiz/curseforge/packinterop"
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

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import [modpack]",
	Short: "Import a curseforge modpack, from a download URL or a downloaded pack zip, or an installed metadata json file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]
		var packImport packinterop.ImportPackMetadata

		// TODO: refactor/extract file checking?
		if strings.HasPrefix(inputFile, "http") {
			// TODO: implement
			fmt.Println("HTTP not supported (yet)")
			os.Exit(1)
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
				// Read the whole file (as bufio doesn't work for zips)
				zipData, err := ioutil.ReadAll(buf)
				if err != nil {
					fmt.Printf("Error reading file: %s\n", err)
					os.Exit(1)
				}
				// Get zip size
				stat, err := f.Stat()
				if err != nil {
					fmt.Printf("Error reading file: %s\n", err)
					os.Exit(1)
				}
				zr, err := zip.NewReader(bytes.NewReader(zipData), stat.Size())
				if err != nil {
					fmt.Printf("Error parsing zip: %s\n", err)
					os.Exit(1)
				}

				// Search the zip for minecraftinstance.json or manifest.json
				var metaFile *zip.File
				for _, v := range zr.File {
					if v.Name == "minecraftinstance.json" || v.Name == "manifest.json" {
						metaFile = v
					}
				}

				if metaFile == nil {
					fmt.Println("Can't find manifest.json or minecraftinstance.json, is this a valid pack?")
					os.Exit(1)
				}

				packImport = packinterop.ReadMetadata(packinterop.GetZipPackSource(metaFile, zr))
			} else {
				packImport = packinterop.ReadMetadata(packinterop.GetDiskPackSource(buf, filepath.ToSlash(filepath.Base(inputFile)), filepath.Dir(inputFile)))
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
			modIDs[i] = v.ProjectID
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
			modInfoValue, ok := modInfosMap[v.ProjectID]
			if !ok {
				fmt.Printf("Failed to obtain mod information for ID %d\n", v.ProjectID)
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
				fileInfo, err = getFileInfo(v.ProjectID, v.FileID)
				if err != nil {
					fmt.Printf("Failed to obtain file information for Mod / File %d / %d: %s\n", v.ProjectID, v.FileID, err)
					continue
				}
			}

			err = createModFile(modInfoValue, fileInfo, &index)
			if err != nil {
				fmt.Printf("Failed to save mod \"%s\": %s\n", modInfoValue.Name, err)
				os.Exit(1)
			}

			// TODO: just use mods-folder directly? does texture pack importing affect this?
			ref, err := filepath.Abs(filepath.Join(filepath.Dir(core.ResolveMod(modInfoValue.Slug)), fileInfo.FileName))
			if err == nil {
				referencedModPaths = append(referencedModPaths, ref)
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
			filePath := filepath.Join(indexFolder, filepath.FromSlash(v.Name()))
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
				if v.Name() == "minecraftinstance.json" {
					fmt.Println("Ignored file \"minecraftinstance.json\"")
					successes++
					continue
				}
				if v.Name() == "manifest.json" {
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
