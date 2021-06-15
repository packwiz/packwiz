package curseforge

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/viper"
	"os"
	"strings"

	"github.com/comp500/packwiz/core"
	"github.com/spf13/cobra"
	"gopkg.in/dixonwille/wmenu.v4"
)

const maxCycles = 20

type installableDep struct {
	modInfo
	fileInfo modFileInfo
}

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "install [mod]",
	Short:   "Install a mod from a curseforge URL, slug, ID or search",
	Aliases: []string{"add", "get"},
	Args:    cobra.ArbitraryArgs,
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
		mcVersion, err := pack.GetMCVersion()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var done bool
		var modID, fileID int
		// If mod/file IDs are provided in command line, use those
		// TODO: use file id with slug if it exists?
		if addonIDFlag != 0 {
			modID = addonIDFlag
			fileID = fileIDFlag
			done = true
		}
		if (len(args) == 0 || len(args[0]) == 0) && !done {
			fmt.Println("You must specify a mod.")
			os.Exit(1)
		}
		// If there are more than 1 argument, go straight to searching - URLs/Slugs should not have spaces!
		if !done && len(args) == 1 {
			done, modID, fileID, err = getFileIDsFromString(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			if !done {
				done, modID, err = getModIDFromString(args[0])
				// Ignore error, go to search instead (e.g. lowercase to search instead of as a slug)
				if err != nil {
					done = false
				}
			}
		}

		modInfoObtained := false
		var modInfoData modInfo

		if !done {
			var cancelled bool
			cancelled, modInfoData = searchCurseforgeInternal(args, mcVersion, getLoader(pack))
			if cancelled {
				return
			}
			done = true
			modID = modInfoData.ID
			modInfoObtained = true
		}

		if !done {
			if err == nil {
				fmt.Println("No mods found!")
				os.Exit(1)
			}
			fmt.Println(err)
			os.Exit(1)
		}

		if !modInfoObtained {
			modInfoData, err = getModInfo(modID)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		var fileInfoData modFileInfo
		fileInfoData, err = getLatestFile(modInfoData, mcVersion, fileID, getLoader(pack))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(fileInfoData.Dependencies) > 0 {
			var depsInstallable []installableDep
			var depIDPendingQueue []int
			for _, dep := range fileInfoData.Dependencies {
				if dep.Type == dependencyTypeRequired {
					depIDPendingQueue = append(depIDPendingQueue, dep.ModID)
				}
			}

			if len(depIDPendingQueue) > 0 {
				fmt.Println("Finding dependencies...")

				cycles := 0
				var installedIDList []int
				for len(depIDPendingQueue) > 0 && cycles < maxCycles {
					if installedIDList == nil {
						// Get modids of all mods
						for _, modPath := range index.GetAllMods() {
							mod, err := core.LoadMod(modPath)
							if err == nil {
								data, ok := mod.GetParsedUpdateData("curseforge")
								if ok {
									updateData, ok := data.(cfUpdateData)
									if ok {
										if updateData.ProjectID > 0 {
											installedIDList = append(installedIDList, updateData.ProjectID)
										}
									}
								}
							}
						}
					}

					// Remove installed IDs from dep queue
					i := 0
					for _, id := range depIDPendingQueue {
						contains := false
						for _, id2 := range installedIDList {
							if id == id2 {
								contains = true
								break
							}
						}
						for _, data := range depsInstallable {
							if id == data.ID {
								contains = true
								break
							}
						}
						if !contains {
							depIDPendingQueue[i] = id
							i++
						}
					}
					depIDPendingQueue = depIDPendingQueue[:i]

					depInfoData, err := getModInfoMultiple(depIDPendingQueue)
					if err != nil {
						fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
					}
					depIDPendingQueue = depIDPendingQueue[:0]

					for _, currData := range depInfoData {
						depFileInfo, err := getLatestFile(currData, mcVersion, 0, getLoader(pack))
						if err != nil {
							fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
							continue
						}

						for _, dep := range depFileInfo.Dependencies {
							if dep.Type == dependencyTypeRequired {
								depIDPendingQueue = append(depIDPendingQueue, dep.ModID)
							}
						}

						depsInstallable = append(depsInstallable, installableDep{
							currData, depFileInfo,
						})
					}

					cycles++
				}
				if cycles >= maxCycles {
					fmt.Println("Dependencies recurse too deeply! Try increasing maxCycles.")
					os.Exit(1)
				}

				if len(depsInstallable) > 0 {
					fmt.Println("Dependencies found:")
					for _, v := range depsInstallable {
						fmt.Println(v.Name)
					}

					fmt.Print("Would you like to install them? [Y/n]: ")
					answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}

					ansNormal := strings.ToLower(strings.TrimSpace(answer))
					if !(len(ansNormal) > 0 && ansNormal[0] == 'n') {
						for _, v := range depsInstallable {
							err = createModFile(v.modInfo, v.fileInfo, &index)
							if err != nil {
								fmt.Println(err)
								os.Exit(1)
							}
							fmt.Printf("Dependency \"%s\" successfully installed! (%s)\n", v.modInfo.Name, v.fileInfo.FileName)
						}
					}
				} else {
					fmt.Println("All dependencies are already installed!")
				}
			}
		}

		err = createModFile(modInfoData, fileInfoData, &index)
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

		fmt.Printf("Mod \"%s\" successfully installed! (%s)\n", modInfoData.Name, fileInfoData.FileName)
	},
}

// Used to implement interface for fuzzy matching
type modResultsList []modInfo

func (r modResultsList) String(i int) string {
	return r[i].Name
}

func (r modResultsList) Len() int {
	return len(r)
}

func searchCurseforgeInternal(args []string, mcVersion string, packLoaderType int) (bool, modInfo) {
	fmt.Println("Searching CurseForge...")
	searchTerm := strings.Join(args, " ")

	// If there are more than one acceptable version, we shouldn't filter by game version at all (as we can't filter by multiple)
	filterGameVersion := getCurseforgeVersion(mcVersion)
	if len(viper.GetStringSlice("acceptable-game-versions")) > 0 {
		filterGameVersion = ""
	}
	results, err := getSearch(searchTerm, filterGameVersion, packLoaderType)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Println("No mods found!")
		os.Exit(1)
		return false, modInfo{}
	} else if len(results) == 1 {
		return false, results[0]
	} else {
		// Fuzzy search on results list
		fuzzySearchResults := fuzzy.FindFrom(searchTerm, modResultsList(results))

		menu := wmenu.NewMenu("Choose a number:")

		menu.Option("Cancel", nil, false, nil)
		if len(fuzzySearchResults) == 0 {
			for i, v := range results {
				menu.Option(v.Name, v, i == 0, nil)
			}
		} else {
			for i, v := range fuzzySearchResults {
				menu.Option(results[v.Index].Name, results[v.Index], i == 0, nil)
			}
		}

		var modInfoData modInfo
		var cancelled bool
		menu.Action(func(menuRes []wmenu.Opt) error {
			if len(menuRes) != 1 || menuRes[0].Value == nil {
				fmt.Println("Cancelled!")
				cancelled = true
				return nil
			}

			// Why is variable shadowing a thing!!!!
			var ok bool
			modInfoData, ok = menuRes[0].Value.(modInfo)
			if !ok {
				return errors.New("error converting interface from wmenu")
			}
			return nil
		})
		err = menu.Run()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if cancelled {
			return true, modInfo{}
		}
		return false, modInfoData
	}
}

func getLatestFile(modInfoData modInfo, mcVersion string, fileID int, packLoaderType int) (modFileInfo, error) {
	// For snapshots, curseforge doesn't put them in GameVersionLatestFiles
	if fileID == 0 {
		var fileInfoData modFileInfo
		fileInfoObtained := false

		for _, v := range modInfoData.LatestFiles {
			// Choose "newest" version by largest ID
			if matchGameVersions(mcVersion, v.GameVersions) && v.ID > fileID && matchLoaderTypeFileInfo(packLoaderType, v) {
				fileID = v.ID
				fileInfoData = v
				fileInfoObtained = true
			}
		}
		// TODO: change to timestamp-based comparison??
		for _, v := range modInfoData.GameVersionLatestFiles {
			// Choose "newest" version by largest ID
			if matchGameVersion(mcVersion, v.GameVersion) && v.ID > fileID && matchLoaderType(packLoaderType, v.Modloader) {
				fileID = v.ID
				fileInfoObtained = false // Make sure we get the file info
			}
		}
		if fileInfoObtained {
			return fileInfoData, nil
		}
	}

	if fileID == 0 {
		return modFileInfo{}, errors.New("mod not available for the configured Minecraft version(s) (use the acceptable-game-versions option to accept more) or loader")
	}

	fileInfoData, err := getFileInfo(modInfoData.ID, fileID)
	if err != nil {
		return modFileInfo{}, err
	}
	return fileInfoData, nil
}

var addonIDFlag int
var fileIDFlag int

func init() {
	curseforgeCmd.AddCommand(installCmd)

	installCmd.Flags().IntVar(&addonIDFlag, "addon-id", 0, "The curseforge addon ID to use")
	installCmd.Flags().IntVar(&fileIDFlag, "file-id", 0, "The curseforge file ID to use")
}
