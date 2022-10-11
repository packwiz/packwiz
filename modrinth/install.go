package modrinth

import (
	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"errors"
	"fmt"
	"github.com/packwiz/packwiz/cmdshared"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"gopkg.in/dixonwille/wmenu.v4"
)

var modSiteRegex = regexp.MustCompile("modrinth\\.com/mod/([^/]+)/?.*$")
var versionSiteRegex = regexp.MustCompile("modrinth\\.com/mod/([^/]+)/version/([^/]+)/?$")

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "install [mod]",
	Short:   "Install a mod from a modrinth URL, slug, ID or search",
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

		if len(args) == 0 || len(args[0]) == 0 {
			fmt.Println("You must specify a mod.")
			os.Exit(1)
		}

		// If there are more than 1 argument, go straight to searching - URLs/Slugs should not have spaces!
		if len(args) > 1 {
			err = installViaSearch(strings.Join(args, " "), pack, &index)
			if err != nil {
				fmt.Printf("Failed installing mod: %s\n", err)
				os.Exit(1)
			}
			return
		}

		//Try interpreting the arg as a version url
		matches := versionSiteRegex.FindStringSubmatch(args[0])
		if matches != nil && len(matches) == 3 {
			err = installVersionById(matches[2], pack, &index)
			if err != nil {
				fmt.Printf("Failed installing mod: %s\n", err)
				os.Exit(1)
			}
			return
		}

		//Try interpreting the arg as a modId or slug.
		//Modrinth transparently handles slugs/mod ids in their api; we don't have to detect which one it is.
		var modStr string

		//Try to see if it's a site, if extract the id/slug from the url.
		//Otherwise, interpret the arg as a id/slug straight up
		matches = modSiteRegex.FindStringSubmatch(args[0])
		if matches != nil && len(matches) == 2 {
			modStr = matches[1]
		} else {
			modStr = args[0]
		}

		mod, err := mrDefaultClient.Projects.Get(modStr)

		if err == nil {
			//We found a mod with that id/slug
			err = installMod(mod, pack, &index)
			if err != nil {
				fmt.Printf("Failed installing mod: %s\n", err)
				os.Exit(1)
			}
			return
		} else {
			//This wasn't a valid modid/slug, try to search for it instead:
			//Don't bother to search if it looks like a url though
			if matches == nil {
				err = installViaSearch(args[0], pack, &index)
				if err != nil {
					fmt.Printf("Failed installing mod: %s\n", err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("Failed installing mod: %s\n", err)
				os.Exit(1)
			}
		}
	},
}

func installViaSearch(query string, pack core.Pack, index *core.Index) error {
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return err
	}

	results, err := getModIdsViaSearch(query, append([]string{mcVersion}, viper.GetStringSlice("acceptable-game-versions")...))
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return errors.New("no results found")
	}

	if viper.GetBool("non-interactive") || len(results) == 1 {
		//Install the first mod
		mod, err := mrDefaultClient.Projects.Get(*results[0].ProjectID)
		if err != nil {
			return err
		}

		return installMod(mod, pack, index)
	}

	//Create menu for the user to choose the correct mod
	menu := wmenu.NewMenu("Choose a number:")
	menu.Option("Cancel", nil, false, nil)
	for i, v := range results {
		// Should be non-nil (Title is a required field)
		menu.Option(*v.Title, v, i == 0, nil)
	}

	menu.Action(func(menuRes []wmenu.Opt) error {
		if len(menuRes) != 1 || menuRes[0].Value == nil {
			return errors.New("Cancelled!")
		}

		//Get the selected mod
		selectedMod, ok := menuRes[0].Value.(*modrinthApi.SearchResult)
		if !ok {
			return errors.New("error converting interface from wmenu")
		}

		//Install the selected mod
		mod, err := mrDefaultClient.Projects.Get(*selectedMod.ProjectID)
		if err != nil {
			return err
		}

		return installMod(mod, pack, index)
	})

	return menu.Run()
}

func installMod(mod *modrinthApi.Project, pack core.Pack, index *core.Index) error {
	fmt.Printf("Found mod %s: '%s'.\n", *mod.Title, *mod.Description)

	latestVersion, err := getLatestVersion(*mod.ID, pack)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}
	if latestVersion.ID == nil {
		return errors.New("mod is not available for this Minecraft version (use the acceptable-game-versions option to accept more) or mod loader")
	}

	return installVersion(mod, latestVersion, pack, index)
}

const maxCycles = 20

type depMetadataStore struct {
	projectInfo *modrinthApi.Project
	versionInfo *modrinthApi.Version
	fileInfo    *modrinthApi.File
}

func installVersion(mod *modrinthApi.Project, version *modrinthApi.Version, pack core.Pack, index *core.Index) error {
	if len(version.Files) == 0 {
		return errors.New("version doesn't have any files attached")
	}

	if len(version.Dependencies) > 0 {
		// TODO: could get installed version IDs, and compare to install the newest - i.e. preferring pinned versions over getting absolute latest?
		installedProjects := getInstalledProjectIDs(index)

		var depMetadata []depMetadataStore
		var depProjectIDPendingQueue []string
		var depVersionIDPendingQueue []string

		for _, dep := range version.Dependencies {
			// TODO: recommend optional dependencies?
			if dep.DependencyType != nil && *dep.DependencyType == "required" {
				if dep.ProjectID != nil {
					depProjectIDPendingQueue = append(depProjectIDPendingQueue, *dep.ProjectID)
				}
				if dep.VersionID != nil {
					depVersionIDPendingQueue = append(depVersionIDPendingQueue, *dep.VersionID)
				}
			}
		}

		if len(depProjectIDPendingQueue)+len(depVersionIDPendingQueue) > 0 {
			fmt.Println("Finding dependencies...")

			cycles := 0
			for len(depProjectIDPendingQueue)+len(depVersionIDPendingQueue) > 0 && cycles < maxCycles {
				// Look up version IDs
				if len(depVersionIDPendingQueue) > 0 {
					depVersions, err := mrDefaultClient.Versions.GetMultiple(depVersionIDPendingQueue)
					if err == nil {
						for _, v := range depVersions {
							// Add project ID to queue
							depProjectIDPendingQueue = append(depProjectIDPendingQueue, *v.ProjectID)
						}
					} else {
						fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
					}
					depVersionIDPendingQueue = depVersionIDPendingQueue[:0]
				}

				// Remove installed project IDs from dep queue
				i := 0
				for _, id := range depProjectIDPendingQueue {
					contains := slices.Contains(installedProjects, id)
					for _, dep := range depMetadata {
						if *dep.projectInfo.ID == id {
							contains = true
							break
						}
					}
					if !contains {
						depProjectIDPendingQueue[i] = id
						i++
					}
				}
				depProjectIDPendingQueue = depProjectIDPendingQueue[:i]

				if len(depProjectIDPendingQueue) == 0 {
					break
				}
				depProjects, err := mrDefaultClient.Projects.GetMultiple(depProjectIDPendingQueue)
				if err != nil {
					fmt.Printf("Error retrieving dependency data: %s\n", err.Error())
				}
				depProjectIDPendingQueue = depProjectIDPendingQueue[:0]

				for _, project := range depProjects {
					if project.ID == nil {
						return errors.New("failed to get dependency data: invalid response")
					}
					// Get latest version - could reuse version lookup data but it's not as easy (particularly since the version won't necessarily be the latest)
					latestVersion, err := getLatestVersion(*project.ID, pack)
					if err != nil {
						return fmt.Errorf("failed to get latest version of dependency %v: %v", *project.ID, err)
					}

					for _, dep := range version.Dependencies {
						// TODO: recommend optional dependencies?
						if dep.DependencyType != nil && *dep.DependencyType == "required" {
							if dep.ProjectID != nil {
								depProjectIDPendingQueue = append(depProjectIDPendingQueue, *dep.ProjectID)
							}
							if dep.VersionID != nil {
								depVersionIDPendingQueue = append(depVersionIDPendingQueue, *dep.VersionID)
							}
						}
					}

					// TODO: add some way to allow users to pick which file to install?
					var file = latestVersion.Files[0]
					// Prefer the primary file
					for _, v := range latestVersion.Files {
						if *v.Primary {
							file = v
						}
					}

					depMetadata = append(depMetadata, depMetadataStore{
						projectInfo: project,
						versionInfo: latestVersion,
						fileInfo:    file,
					})
				}

				cycles++
			}
			if cycles >= maxCycles {
				return errors.New("dependencies recurse too deeply, try increasing maxCycles")
			}

			if len(depMetadata) > 0 {
				fmt.Println("Dependencies found:")
				for _, v := range depMetadata {
					fmt.Println(*v.projectInfo.Title)
				}

				if cmdshared.PromptYesNo("Would you like to add them? [Y/n]: ") {
					for _, v := range depMetadata {
						err := createFileMeta(v.projectInfo, v.versionInfo, v.fileInfo, index)
						if err != nil {
							return err
						}
						fmt.Printf("Dependency \"%s\" successfully added! (%s)\n", *v.projectInfo.Title, *v.fileInfo.Filename)
					}
				}
			} else {
				fmt.Println("All dependencies are already added!")
			}
		}
	}

	// TODO: add some way to allow users to pick which file to install?
	var file = version.Files[0]
	// Prefer the primary file
	for _, v := range version.Files {
		if *v.Primary {
			file = v
		}
	}

	//Install the file
	fmt.Printf("Installing %s from version %s\n", *file.Filename, *version.VersionNumber)

	err := createFileMeta(mod, version, file, index)
	if err != nil {
		return err
	}

	err = index.Write()
	if err != nil {
		return err
	}
	err = pack.UpdateIndexHash()
	if err != nil {
		return err
	}
	err = pack.Write()
	if err != nil {
		return err
	}
	return nil
}

func createFileMeta(mod *modrinthApi.Project, version *modrinthApi.Version, file *modrinthApi.File, index *core.Index) error {
	updateMap := make(map[string]map[string]interface{})

	var err error
	updateMap["modrinth"], err = mrUpdateData{
		ModID:            *mod.ID,
		InstalledVersion: *version.ID,
	}.ToMap()
	if err != nil {
		return err
	}

	side := getSide(mod)
	if side == "" {
		return errors.New("version doesn't have a side that's supported. Server: " + *mod.ServerSide + " Client: " + *mod.ClientSide)
	}

	algorithm, hash := getBestHash(file)
	if algorithm == "" {
		return errors.New("file doesn't have a hash")
	}

	modMeta := core.Mod{
		Name:     *mod.Title,
		FileName: *file.Filename,
		Side:     side,
		Download: core.ModDownload{
			URL:        *file.URL,
			HashFormat: algorithm,
			Hash:       hash,
		},
		Update: updateMap,
	}
	var path string
	folder := viper.GetString("meta-folder")
	if folder == "" {
		folder = "mods"
	}
	if mod.Slug != nil {
		path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder, *mod.Slug+core.MetaExtension))
	} else {
		path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder, core.SlugifyName(*mod.Title)+core.MetaExtension))
	}

	// If the file already exists, this will overwrite it!!!
	// TODO: Should this be improved?
	// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
	// VCS anyway.

	format, hash, err := modMeta.Write()
	if err != nil {
		return err
	}
	return index.RefreshFileWithHash(path, format, hash, true)
}

func installVersionById(versionId string, pack core.Pack, index *core.Index) error {
	version, err := mrDefaultClient.Versions.Get(versionId)
	if err != nil {
		return fmt.Errorf("failed to fetch version %s: %v", versionId, err)
	}

	mod, err := mrDefaultClient.Projects.Get(*version.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to fetch mod %s: %v", *version.ProjectID, err)
	}

	return installVersion(mod, version, pack, index)
}

func init() {
	modrinthCmd.AddCommand(installCmd)
}
