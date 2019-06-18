package curseforge

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/comp500/packwiz/core"
	"github.com/urfave/cli"
	"gopkg.in/dixonwille/wmenu.v4"
)

const maxCycles = 20

type installableDep struct {
	modInfo
	fileInfo modFileInfo
}

func cmdInstall(flags core.Flags, mod string, modArgsTail []string) error {
	if len(mod) == 0 {
		return cli.NewExitError("You must specify a mod.", 1)
	}
	pack, err := core.LoadPack(flags)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	index, err := pack.LoadIndex()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var done bool
	var modID, fileID int
	// If modArgsTail has anything, go straight to searching - URLs/Slugs should not have spaces!
	if len(modArgsTail) == 0 {
		done, modID, fileID, err = getFileIDsFromString(mod)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if !done {
			done, modID, err = getModIDFromString(mod)
			// Ignore error, go to search instead (e.g. lowercase to search instead of as a slug)
			if err != nil {
				done = false
			}
		}
	}

	modInfoObtained := false
	var modInfoData modInfo

	if !done {
		fmt.Println("Searching CurseForge...")
		modArgs := append([]string{mod}, modArgsTail...)
		searchTerm := strings.Join(modArgs, " ")
		results, err := getSearch(searchTerm, mcVersion)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if len(results) == 0 {
			return cli.NewExitError("No mods found!", 1)
		} else if len(results) == 1 {
			modInfoData = results[0]
			modID = modInfoData.ID
			modInfoObtained = true
			done = true
		} else {
			// Find the closest value to the search term
			sort.Slice(results, func(i, j int) bool {
				return levenshtein.ComputeDistance(searchTerm, results[i].Name) < levenshtein.ComputeDistance(searchTerm, results[j].Name)
			})

			menu := wmenu.NewMenu("Choose a number:")

			for i, v := range results {
				menu.Option(v.Name, v, i == 0, nil)
			}
			menu.Option("Cancel", nil, false, nil)

			menu.Action(func(menuRes []wmenu.Opt) error {
				if len(menuRes) != 1 || menuRes[0].Value == nil {
					fmt.Println("Cancelled!")
					return nil
				}

				// Why is variable shadowing a thing!!!!
				var ok bool
				modInfoData, ok = menuRes[0].Value.(modInfo)
				if !ok {
					return cli.NewExitError("Error converting interface from wmenu", 1)
				}
				modID = modInfoData.ID
				modInfoObtained = true
				done = true
				return nil
			})
			err = menu.Run()
			if err != nil {
				return cli.NewExitError(err, 1)
			}

			if !done {
				return nil
			}
		}
	}

	if !done {
		if err == nil {
			return cli.NewExitError("No mods found!", 1)
		}
		return cli.NewExitError(err, 1)
	}

	if !modInfoObtained {
		modInfoData, err = getModInfo(modID)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	var fileInfoData modFileInfo
	fileInfoData, err = getLatestFile(modInfoData, mcVersion, fileID)
	if err != nil {
		return cli.NewExitError(err, 1)
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
					depFileInfo, err := getLatestFile(currData, mcVersion, 0)
					if err != nil {
						fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
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
				return cli.NewExitError("Dependencies recurse too deeply! Try increasing maxCycles.", 1)
			}

			if len(depsInstallable) > 0 {
				fmt.Println("Dependencies found:")
				for _, v := range depsInstallable {
					fmt.Println(v.Name)
				}

				fmt.Print("Would you like to install them? [Y/n]: ")
				answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return cli.NewExitError(err, 1)
				}

				ansNormal := strings.ToLower(strings.TrimSpace(answer))
				if !(len(ansNormal) > 0 && ansNormal[0] == 'n') {
					for _, v := range depsInstallable {
						err = createModFile(flags, v.modInfo, v.fileInfo, &index)
						if err != nil {
							return cli.NewExitError(err, 1)
						}
						fmt.Printf("Dependency \"%s\" successfully installed! (%s)\n", v.modInfo.Name, v.fileInfo.FileName)
					}
				}
			} else {
				fmt.Println("All dependencies are already installed!")
			}
		}
	}

	err = createModFile(flags, modInfoData, fileInfoData, &index)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	err = index.Write()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = pack.UpdateIndexHash()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = pack.Write()
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Mod \"%s\" successfully installed! (%s)\n", modInfoData.Name, fileInfoData.FileName)

	return nil
}

func getLatestFile(modInfoData modInfo, mcVersion string, fileID int) (modFileInfo, error) {
	if fileID == 0 {
		// TODO: change to timestamp-based comparison??
		for _, v := range modInfoData.GameVersionLatestFiles {
			// Choose "newest" version by largest ID
			if v.GameVersion == mcVersion && v.ID > fileID {
				fileID = v.ID
			}
		}
	}

	if fileID == 0 {
		return modFileInfo{}, errors.New("mod not available for this minecraft version")
	}

	// The API also provides some files inline, because that's efficient!
	for _, v := range modInfoData.LatestFiles {
		if v.ID == fileID {
			return v, nil
		}
	}

	fileInfoData, err := getFileInfo(modInfoData.ID, fileID)
	if err != nil {
		return modFileInfo{}, err
	}
	return fileInfoData, nil
}
