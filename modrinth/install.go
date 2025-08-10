package modrinth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"github.com/packwiz/packwiz/cmdshared"
	"github.com/spf13/viper"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"gopkg.in/dixonwille/wmenu.v4"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "add [URL|slug|search]",
	Short:   "Add a project from a Modrinth URL, slug/project ID or search",
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

		// If project/version IDs/version file name is provided in command line, use those
		var projectID, versionID, versionFilename string
		if projectIDFlag != "" {
			projectID = projectIDFlag
			if len(args) != 0 {
				fmt.Println("--project-id cannot be used with a separately specified URL/slug/search term")
				os.Exit(1)
			}
		}
		if versionIDFlag != "" {
			versionID = versionIDFlag
			if len(args) != 0 {
				fmt.Println("--version-id cannot be used with a separately specified URL/slug/search term")
				os.Exit(1)
			}
		}
		if versionFilenameFlag != "" {
			versionFilename = versionFilenameFlag
		}

		if (len(args) == 0 || len(args[0]) == 0) && projectID == "" {
			fmt.Println("You must specify a project; with the ID flags, or by passing a URL, slug or search term directly.")
			os.Exit(1)
		}

		var version string
		var parsedSlug bool
		if projectID == "" && versionID == "" && len(args) == 1 {
			// Try interpreting the argument as a slug/project ID, or project/version/CDN URL
			parsedSlug, err = parseSlugOrUrl(args[0], &projectID, &version, &versionID, &versionFilename)
			if err != nil {
				fmt.Printf("Failed to parse URL: %v\n", err)
				os.Exit(1)
			}
		}

		// Got version ID; install using this ID
		if versionID != "" {
			err = installVersionById(versionID, versionFilename, pack, &index)
			if err != nil {
				fmt.Printf("Failed to add project: %s\n", err)
				os.Exit(1)
			}
			return
		}

		// Look up project ID
		if projectID != "" {
			// Modrinth transparently handles slugs/project IDs in their API; we don't have to detect which one it is.
			var project *modrinthApi.Project
			project, err = mrDefaultClient.Projects.Get(projectID)
			if err == nil {
				// We found a project with that id/slug
				if version != "" {
					// Try to look up version number
					versionData, err := resolveVersion(project, version)
					if err != nil {
						fmt.Printf("Failed to add project: %s\n", err)
						os.Exit(1)
					}
					err = installVersion(project, versionData, versionFilename, pack, &index)
					if err != nil {
						fmt.Printf("Failed to add project: %s\n", err)
						os.Exit(1)
					}
					return
				}

				// No version specified; find latest
				err = installProject(project, versionFilename, pack, &index)
				if err != nil {
					fmt.Printf("Failed to add project: %s\n", err)
					os.Exit(1)
				}
				return
			}
		}

		// Arguments weren't a valid slug/project ID, try to search for it instead (if it was not parsed as a URL)
		if projectID == "" || parsedSlug {
			err = installViaSearch(strings.Join(args, " "), versionFilename, !parsedSlug, pack, &index)
			if err != nil {
				fmt.Printf("Failed to add project: %s\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("Failed to add project: %s\n", err)
			os.Exit(1)
		}
	},
}

func installVersionById(versionId string, versionFilename string, pack core.Pack, index *core.Index) error {
	version, err := mrDefaultClient.Versions.Get(versionId)
	if err != nil {
		return fmt.Errorf("failed to fetch version %s: %v", versionId, err)
	}

	project, err := mrDefaultClient.Projects.Get(*version.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to fetch project %s: %v", *version.ProjectID, err)
	}

	return installVersion(project, version, versionFilename, pack, index)
}

func installViaSearch(query string, versionFilename string, autoAcceptFirst bool, pack core.Pack, index *core.Index) error {
	mcVersions, err := pack.GetSupportedMCVersions()
	if err != nil {
		return err
	}

	fmt.Println("Searching Modrinth...")

	results, err := getProjectIdsViaSearch(query, mcVersions)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return errors.New("no projects found")
	}

	if viper.GetBool("non-interactive") || (len(results) == 1 && autoAcceptFirst) {
		// Install the first project found
		project, err := mrDefaultClient.Projects.Get(*results[0].ProjectID)
		if err != nil {
			return err
		}

		return installProject(project, versionFilename, pack, index)
	}

	// Create menu for the user to choose the correct project
	menu := wmenu.NewMenu("Choose a number:")
	menu.Option("Cancel", nil, false, nil)
	for i, v := range results {
		// Should be non-nil (Title is a required field)
		menu.Option(*v.Title, v, i == 0, nil)
	}

	menu.Action(func(menuRes []wmenu.Opt) error {
		if len(menuRes) != 1 || menuRes[0].Value == nil {
			return errors.New("project selection cancelled")
		}

		// Get the selected project
		selectedProject, ok := menuRes[0].Value.(*modrinthApi.SearchResult)
		if !ok {
			return errors.New("error converting interface from wmenu")
		}

		// Install the selected project
		project, err := mrDefaultClient.Projects.Get(*selectedProject.ProjectID)
		if err != nil {
			return err
		}

		return installProject(project, versionFilename, pack, index)
	})

	return menu.Run()
}

func installProject(project *modrinthApi.Project, versionFilename string, pack core.Pack, index *core.Index) error {
	latestVersion, err := getLatestVersion(*project.ID, *project.Title, pack)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}
	if latestVersion.ID == nil {
		return errors.New("mod not available for the configured Minecraft version(s) (use the 'packwiz settings acceptable-versions' command to accept more) or loader")
	}

	return installVersion(project, latestVersion, versionFilename, pack, index)
}

const maxCycles = 20

type depMetadataStore struct {
	projectInfo *modrinthApi.Project
	versionInfo *modrinthApi.Version
	fileInfo    *modrinthApi.File
}

func installVersion(project *modrinthApi.Project, version *modrinthApi.Version, versionFilename string, pack core.Pack, index *core.Index) error {
	if len(version.Files) == 0 {
		return errors.New("version doesn't have any files attached")
	}

	if len(version.Dependencies) > 0 {
		// TODO: could get installed version IDs, and compare to install the newest - i.e. preferring pinned versions over getting absolute latest?
		installedProjects := getInstalledProjectIDs(index)
		isQuilt := slices.Contains(pack.GetCompatibleLoaders(), "quilt")
		mcVersion, err := pack.GetMCVersion()
		if err != nil {
			return err
		}

		var depMetadata []depMetadataStore
		var depProjectIDPendingQueue []string
		var depVersionIDPendingQueue []string

		for _, dep := range version.Dependencies {
			// TODO: recommend optional dependencies?
			if dep.DependencyType != nil && *dep.DependencyType == "required" {
				if dep.VersionID != nil {
					depVersionIDPendingQueue = append(depVersionIDPendingQueue, *dep.VersionID)
				} else {
					if dep.ProjectID != nil {
						depProjectIDPendingQueue = append(depProjectIDPendingQueue, mapDepOverride(*dep.ProjectID, isQuilt, mcVersion))
					}
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
							depProjectIDPendingQueue = append(depProjectIDPendingQueue, mapDepOverride(*v.ProjectID, isQuilt, mcVersion))
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

				// Clean up duplicates from dep queue (from deps on both QFAPI + FAPI)
				slices.Sort(depProjectIDPendingQueue)
				depProjectIDPendingQueue = slices.Compact(depProjectIDPendingQueue)

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
					latestVersion, err := getLatestVersion(*project.ID, *project.Title, pack)
					if err != nil {
						fmt.Printf("Failed to get latest version of dependency %v: %v\n", *project.Title, err)
						continue
					}

					for _, dep := range version.Dependencies {
						// TODO: recommend optional dependencies?
						if dep.DependencyType != nil && *dep.DependencyType == "required" {
							if dep.ProjectID != nil {
								depProjectIDPendingQueue = append(depProjectIDPendingQueue, mapDepOverride(*dep.ProjectID, isQuilt, mcVersion))
							}
							if dep.VersionID != nil {
								depVersionIDPendingQueue = append(depVersionIDPendingQueue, *dep.VersionID)
							}
						}
					}

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
						err := createFileMeta(v.projectInfo, v.versionInfo, v.fileInfo, pack, index)
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

	var file = version.Files[0]
	// Prefer the primary file
	for _, v := range version.Files {
		if (*v.Primary) || (versionFilename != "" && versionFilename == *v.Filename) {
			file = v
		}
	}
	// TODO: handle optional/required resource pack files

	// Create the metadata file
	err := createFileMeta(project, version, file, pack, index)
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

	fmt.Printf("Project \"%s\" successfully added! (%s)\n", *project.Title, *file.Filename)
	return nil
}

func createFileMeta(project *modrinthApi.Project, version *modrinthApi.Version, file *modrinthApi.File, pack core.Pack, index *core.Index) error {
	updateMap := make(map[string]map[string]interface{})

	var err error
	updateMap["modrinth"], err = mrUpdateData{
		ProjectID:        *project.ID,
		InstalledVersion: *version.ID,
	}.ToMap()
	if err != nil {
		return err
	}

	side := getSide(project)
	if side == "" {
		fmt.Println("Warning: Project doesn't have a side that's supported; assuming universal. Server: " + *project.ServerSide + " Client: " + *project.ClientSide)
		side = core.UniversalSide
	}

	algorithm, hash := getBestHash(file)
	if algorithm == "" {
		return errors.New("file doesn't have a hash")
	}

	modMeta := core.Mod{
		Name:     *project.Title,
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
		folder, err = getProjectTypeFolder(*project.ProjectType, version.Loaders, pack.GetCompatibleLoaders())
		if err != nil {
			return err
		}
	}
	if project.Slug != nil {
		path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder, *project.Slug+core.MetaExtension))
	} else {
		path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder, core.SlugifyName(*project.Title)+core.MetaExtension))
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

var projectIDFlag string
var versionIDFlag string
var versionFilenameFlag string

func init() {
	modrinthCmd.AddCommand(installCmd)

	installCmd.Flags().StringVar(&projectIDFlag, "project-id", "", "The Modrinth project ID to use")
	installCmd.Flags().StringVar(&versionIDFlag, "version-id", "", "The Modrinth version ID to use")
	installCmd.Flags().StringVar(&versionFilenameFlag, "version-filename", "", "The Modrinth version filename to use")
}
