package curseforge

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/packwiz/packwiz/cmdshared"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/viper"

	"github.com/packwiz/packwiz/core"
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
	Use:     "add [URL|slug|search]",
	Short:   "Add a project from a CurseForge URL, slug, ID or search",
	Aliases: []string{"install", "get"},
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
		mcVersions, err := pack.GetSupportedMCVersions()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		primaryMCVersion, err := pack.GetMCVersion()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		game := gameFlag
		category := categoryFlag
		var modID, fileID uint32
		var slug string

		// If mod/file IDs are provided in command line, use those
		if fileIDFlag != 0 {
			fileID = fileIDFlag
		}
		if addonIDFlag != 0 {
			modID = addonIDFlag
		}

		if (len(args) == 0 || len(args[0]) == 0) && modID == 0 {
			fmt.Println("You must specify a project; with the ID flags, or by passing a URL, slug or search term directly.")
			os.Exit(1)
		}
		if modID == 0 && len(args) == 1 {
			parsedGame, parsedCategory, parsedSlug, parsedFileID, err := parseSlugOrUrl(args[0])
			if err != nil {
				fmt.Printf("Failed to parse URL: %v\n", err)
				os.Exit(1)
			}

			if parsedGame != "" {
				game = parsedGame
			}
			if parsedCategory != "" {
				category = parsedCategory
			}
			if parsedSlug != "" {
				slug = parsedSlug
			}
			if parsedFileID != 0 {
				fileID = parsedFileID
			}
		}

		modInfoObtained := false
		var modInfoData modInfo

		if modID == 0 {
			var cancelled bool
			if slug == "" {
				searchTerm := strings.Join(args, " ")
				cancelled, modInfoData = searchCurseforgeInternal(searchTerm, false, game, category, mcVersions, getSearchLoaderType(pack))
			} else {
				cancelled, modInfoData = searchCurseforgeInternal(slug, true, game, category, mcVersions, getSearchLoaderType(pack))
			}
			if cancelled {
				return
			}
			modID = modInfoData.ID
			modInfoObtained = true
		}

		if modID == 0 {
			fmt.Println("No projects found!")
			os.Exit(1)
		}

		if !modInfoObtained {
			modInfoData, err = cfDefaultClient.getModInfo(modID)
			if err != nil {
				fmt.Printf("Failed to get project info: %v\n", err)
				os.Exit(1)
			}
		}

		var fileInfoData modFileInfo
		fileInfoData, err = getLatestFile(modInfoData, mcVersions, fileID, pack.GetCompatibleLoaders())
		if err != nil {
			fmt.Printf("Failed to get file for project: %v\n", err)
			os.Exit(1)
		}

		if len(fileInfoData.Dependencies) > 0 {
			isQuilt := slices.Contains(pack.GetCompatibleLoaders(), "quilt")

			var depsInstallable []installableDep
			var depIDPendingQueue []uint32
			for _, dep := range fileInfoData.Dependencies {
				if dep.Type == dependencyTypeRequired {
					depIDPendingQueue = append(depIDPendingQueue, mapDepOverride(dep.ModID, isQuilt, primaryMCVersion))
				}
			}

			if len(depIDPendingQueue) > 0 {
				fmt.Println("Finding dependencies...")

				cycles := 0
				var installedIDList []uint32
				for len(depIDPendingQueue) > 0 && cycles < maxCycles {
					if installedIDList == nil {
						// Get modids of all mods
						mods, err := index.LoadAllMods()
						if err != nil {
							fmt.Printf("Failed to determine existing projects: %v\n", err)
						} else {
							for _, mod := range mods {
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
						contains := slices.Contains(installedIDList, id)
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

					if len(depIDPendingQueue) == 0 {
						break
					}

					depInfoData, err := cfDefaultClient.getModInfoMultiple(depIDPendingQueue)
					if err != nil {
						fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
					}
					depIDPendingQueue = depIDPendingQueue[:0]

					for _, currData := range depInfoData {
						depFileInfo, err := getLatestFile(currData, mcVersions, 0, pack.GetCompatibleLoaders())
						if err != nil {
							fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
							continue
						}

						for _, dep := range depFileInfo.Dependencies {
							if dep.Type == dependencyTypeRequired {
								depIDPendingQueue = append(depIDPendingQueue, mapDepOverride(dep.ModID, isQuilt, primaryMCVersion))
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

					if cmdshared.PromptYesNo("Would you like to add them? [Y/n]: ") {
						for _, v := range depsInstallable {
							err = createModFile(v.modInfo, v.fileInfo, &index, false)
							if err != nil {
								fmt.Println(err)
								os.Exit(1)
							}
							fmt.Printf("Dependency \"%s\" successfully added! (%s)\n", v.modInfo.Name, v.fileInfo.FileName)
						}
					}
				} else {
					fmt.Println("All dependencies are already added!")
				}
			}
		}

		err = createModFile(modInfoData, fileInfoData, &index, false)
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

		fmt.Printf("Project \"%s\" successfully added! (%s)\n", modInfoData.Name, fileInfoData.FileName)
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

func searchCurseforgeInternal(searchTerm string, isSlug bool, game string, category string, mcVersions []string, searchLoaderType modloaderType) (bool, modInfo) {
	if isSlug {
		fmt.Println("Looking up CurseForge slug...")
	} else {
		fmt.Println("Searching CurseForge...")
	}

	var gameID, categoryID, classID uint32
	if game == "minecraft" {
		gameID = 432
	}
	if category == "mc-mods" {
		classID = 6
	}
	if gameID == 0 {
		games, err := cfDefaultClient.getGames()
		if err != nil {
			fmt.Printf("Failed to lookup game %s: %v\n", game, err)
			os.Exit(1)
		}
		for _, v := range games {
			if v.Slug == game {
				if v.Status != gameStatusLive {
					fmt.Printf("Failed to lookup game %s: selected game is not live!\n", game)
					os.Exit(1)
				}
				if v.APIStatus != gameApiStatusPublic {
					fmt.Printf("Failed to lookup game %s: selected game does not have a public API!\n", game)
					os.Exit(1)
				}
				gameID = v.ID
				break
			}
		}
		if gameID == 0 {
			fmt.Printf("Failed to lookup: game %s could not be found!\n", game)
			os.Exit(1)
		}
	}
	if categoryID == 0 && classID == 0 && category != "" {
		categories, err := cfDefaultClient.getCategories(gameID)
		if err != nil {
			fmt.Printf("Failed to lookup categories: %v\n", err)
			os.Exit(1)
		}
		for _, v := range categories {
			if v.Slug == category {
				if v.IsClass {
					classID = v.ID
				} else {
					classID = v.ClassID
					categoryID = v.ID
				}
				break
			}
		}
		if categoryID == 0 && classID == 0 {
			fmt.Printf("Failed to lookup: category %s could not be found!\n", category)
			os.Exit(1)
		}
	}

	// If there are more than one acceptable version, we shouldn't filter by game version at all (as we can't filter by multiple)
	filterGameVersion := ""
	if len(mcVersions) == 1 {
		filterGameVersion = getCurseforgeVersion(mcVersions[0])
	}
	var search, slug string
	if isSlug {
		slug = searchTerm
	} else {
		search = searchTerm
	}
	results, err := cfDefaultClient.getSearch(search, slug, gameID, classID, categoryID, filterGameVersion, searchLoaderType)
	if err != nil {
		fmt.Printf("Failed to search for project: %v\n", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Println("No projects found!")
		os.Exit(1)
		return false, modInfo{}
	} else if len(results) == 1 {
		return false, results[0]
	} else {
		// Fuzzy search on results list
		fuzzySearchResults := fuzzy.FindFrom(searchTerm, modResultsList(results))

		if viper.GetBool("non-interactive") {
			if len(fuzzySearchResults) > 0 {
				return false, results[fuzzySearchResults[0].Index]
			}
			return false, results[0]
		}

		menu := wmenu.NewMenu("Choose a number:")

		menu.Option("Cancel", nil, false, nil)
		if len(fuzzySearchResults) == 0 {
			for i, v := range results {
				menu.Option(v.Name+" ("+v.Summary+")", v, i == 0, nil)
			}
		} else {
			for i, v := range fuzzySearchResults {
				menu.Option(results[v.Index].Name+" ("+results[v.Index].Summary+")", results[v.Index], i == 0, nil)
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

func getLatestFile(modInfoData modInfo, mcVersions []string, fileID uint32, packLoaders []string) (modFileInfo, error) {
	if fileID == 0 {
		if len(modInfoData.LatestFiles) == 0 && len(modInfoData.GameVersionLatestFiles) == 0 {
			return modFileInfo{}, fmt.Errorf("addon %d has no files", modInfoData.ID)
		}

		var fileInfoData *modFileInfo
		fileID, fileInfoData, _ = findLatestFile(modInfoData, mcVersions, packLoaders)
		if fileInfoData != nil {
			return *fileInfoData, nil
		}

		// Possible to reach this point without obtaining file info; particularly from GameVersionLatestFiles
		if fileID == 0 {
			return modFileInfo{}, errors.New("mod not available for the configured Minecraft version(s) (use the 'packwiz settings acceptable-versions' command to accept more) or loader")
		}
	}

	fileInfoData, err := cfDefaultClient.getFileInfo(modInfoData.ID, fileID)
	if err != nil {
		return modFileInfo{}, err
	}
	return fileInfoData, nil
}

var addonIDFlag uint32
var fileIDFlag uint32

var gameFlag string
var categoryFlag string

func init() {
	curseforgeCmd.AddCommand(installCmd)

	installCmd.Flags().Uint32Var(&addonIDFlag, "addon-id", 0, "The CurseForge project ID to use")
	installCmd.Flags().Uint32Var(&fileIDFlag, "file-id", 0, "The CurseForge file ID to use")
	installCmd.Flags().StringVar(&gameFlag, "game", "minecraft", "The game to add files from (slug, as stored in URLs); the game in the URL takes precedence")
	installCmd.Flags().StringVar(&categoryFlag, "category", "", "The category to add files from (slug, as stored in URLs); the category in the URL takes precedence")
}
