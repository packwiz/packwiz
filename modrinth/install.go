package modrinth

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/comp500/packwiz/core"
	"github.com/spf13/cobra"
	"gopkg.in/dixonwille/wmenu.v4"
)

var modSiteRegex = regexp.MustCompile("modrinth\\.com/mod/([^/]+)/?$")
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

		if len(args) == 0 || len(args[0]) == 0 {
			fmt.Println("You must specify a mod.")
			os.Exit(1)
		}

		// If there are more than 1 argument, go straight to searching - URLs/Slugs should not have spaces!
		if len(args) > 1 {
			err = installViaSearch(strings.Join(args, " "), pack)
			if err != nil {
				fmt.Printf("Failed installing mod: %s\n", err)
				os.Exit(1)
			}
			return
		}

		//Try interpreting the arg as a version url
		matches := versionSiteRegex.FindStringSubmatch(args[0])
		if matches != nil && len(matches) == 3 {
			err = installVersionById(matches[2], pack)
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

		mod, err := fetchMod(modStr)

		if err == nil {
			//We found a mod with that id/slug
			err = installMod(mod, pack)
			if err != nil {
				fmt.Printf("Failed installing mod: %s\n", err)
				os.Exit(1)
			}
			return
		} else {
			//This wasn't a valid modid/slug, try to search for it instead:
			//Don't bother to search if it looks like a url though
			if !strings.Contains(args[0], "modrinth.com") {
				err = installViaSearch(args[0], pack)
				if err != nil {
					fmt.Printf("Failed installing mod: %s\n", err)
					os.Exit(1)
				}
			}
		}
	},
}

func installViaSearch(query string, pack core.Pack) error {
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return err
	}

	results, err := getModIdsViaSearch(query, mcVersion)
	if err != nil {
		return err
	}

	//Create menu for the user to choose the correct mod
    menu := wmenu.NewMenu("Choose a number:")
    for i, v := range results {
        menu.Option(v.Title, v, i == 0, nil)
    }
    menu.Option("Cancel", nil, false, nil)

    menu.Action(func(menuRes []wmenu.Opt) error {
        if len(menuRes) != 1 || menuRes[0].Value == nil {
            return errors.New("Cancelled!")
        }

        //Get the selected mod
        selectedMod, ok := menuRes[0].Value.(ModResult)
        if !ok {
            return errors.New("error converting interface from wmenu")
        }

        //Install the selected mod
        modId := strings.TrimPrefix(selectedMod.ModID, "local-")

        mod, err := fetchMod(modId)
        if err != nil {
            return err
        }

        return installMod(mod, pack)
    })

    return menu.Run()
}

func installMod(mod Mod, pack core.Pack) error {
	fmt.Printf("Found mod %s: '%s'.\n", mod.Title, mod.Description)

	latestVersion, err := getLatestVersion(mod.ID, pack)
	if err != nil {
		return err
	}
	if latestVersion.ID == "" {
		return errors.New("mod is not available for this minecraft version or mod loader")
	}

	return installVersion(mod, latestVersion, pack)
}

func installVersion(mod Mod, version Version, pack core.Pack) error {
	var files = version.Files

	if len(files) == 0 {
		return errors.New("version doesn't have any files attached")
	}

	var file = files[0]

	//Install the file
	fmt.Printf("Installing %s from version %s\n", file.Filename, version.VersionNumber)
	index, err := pack.LoadIndex()
	if err != nil {
		return err
	}

	updateMap := make(map[string]map[string]interface{})

	updateMap["modrinth"], err = mrUpdateData{
		ModID:            mod.ID,
		InstalledVersion: version.ID,
	}.ToMap()
	if err != nil {
		return err
	}

	side := mod.getSide()
	if side == "" {
		return errors.New("version doesn't have a side that's supported. Server: " + mod.ServerSide + " Client: " + mod.ClientSide)
	}

	algorithm, hash := file.getBestHash()
	if algorithm == "" {
		return errors.New("file doesn't have a hash")
	}

	modMeta := core.Mod{
		Name:     mod.Title,
		FileName: file.Filename,
		Side:     side,
		Download: core.ModDownload{
			URL:        file.Url,
			HashFormat: algorithm,
			Hash:       hash,
		},
		Update: updateMap,
	}
	var path string
	if mod.Slug != "" {
		path = modMeta.SetMetaName(mod.Slug)
	} else {
		path = modMeta.SetMetaName(mod.Title)
	}

	// If the file already exists, this will overwrite it!!!
	// TODO: Should this be improved?
	// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
	// VCS anyway.

	format, hash, err := modMeta.Write()
	if err != nil {
		return err
	}
	err = index.RefreshFileWithHash(path, format, hash, true)
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

func installVersionById(versionId string, pack core.Pack) error {
	version, err := fetchVersion(versionId)
	if err != nil {
		return err
	}

	mod, err := fetchMod(version.ModID)
	if err != nil {
		return err
	}

	return installVersion(mod, version, pack)
}

func init() {
	modrinthCmd.AddCommand(installCmd)
}
