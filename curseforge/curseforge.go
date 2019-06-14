package curseforge

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/comp500/packwiz/core"
	"github.com/mitchellh/mapstructure"
	"github.com/urfave/cli"
	"gopkg.in/dixonwille/wmenu.v4"
)

func init() {
	core.Commands = append(core.Commands, cli.Command{
		Name:  "curseforge",
		Usage: "Manage curseforge-based mods",
		Subcommands: []cli.Command{{
			Name:    "install",
			Usage:   "Install a mod from a curseforge URL, slug or ID",
			Aliases: []string{"add", "get"},
			Action: func(c *cli.Context) error {
				return cmdInstall(core.FlagsFromContext(c), c.Args().Get(0), c.Args().Tail())
			},
		}, {
			Name:  "import",
			Usage: "Import an installed curseforge modpack",
			Action: func(c *cli.Context) error {
				return cmdImport(core.FlagsFromContext(c), c.Args().Get(0))
			},
		}},
	})
	core.UpdateParsers["curseforge"] = cfUpdateParser{}
}

var fileIDRegexes = [...]*regexp.Regexp{
	regexp.MustCompile("^https?:\\/\\/minecraft\\.curseforge\\.com\\/projects\\/(.+)\\/files\\/(\\d+)$"),
	regexp.MustCompile("^https?:\\/\\/(?:www\\.)?curseforge\\.com\\/minecraft\\/mc-mods\\/(.+)\\/download\\/(\\d+)$"),
}

func getFileIDsFromString(mod string) (bool, int, int, error) {
	for _, v := range fileIDRegexes {
		matches := v.FindStringSubmatch(mod)
		if matches != nil && len(matches) == 3 {
			modID, err := modIDFromSlug(matches[1])
			fileID, err := strconv.Atoi(matches[2])
			if err != nil {
				return true, 0, 0, err
			}
			return true, modID, fileID, nil
		}
	}
	return false, 0, 0, nil
}

var modSlugRegexes = [...]*regexp.Regexp{
	regexp.MustCompile("^https?:\\/\\/minecraft\\.curseforge\\.com\\/projects\\/([^\\/]+)$"),
	regexp.MustCompile("^https?:\\/\\/(?:www\\.)?curseforge\\.com\\/minecraft\\/mc-mods\\/([^\\/]+)$"),
	// Exact slug matcher
	regexp.MustCompile("^[a-z][\\da-z\\-]{0,127}$"),
}

func getModIDFromString(mod string) (bool, int, error) {
	// Check if it's just a number first
	modID, err := strconv.Atoi(mod)
	if err == nil && modID > 0 {
		return true, modID, nil
	}

	for _, v := range modSlugRegexes {
		matches := v.FindStringSubmatch(mod)
		if matches != nil {
			var slug string
			if len(matches) == 2 {
				slug = matches[1]
			} else if len(matches) == 1 {
				slug = matches[0]
			} else {
				continue
			}
			modID, err := modIDFromSlug(slug)
			if err != nil {
				return true, 0, err
			}
			return true, modID, nil
		}
	}
	return false, 0, nil
}

func createModFile(flags core.Flags, modInfo modInfo, fileInfo modFileInfo, index *core.Index) error {
	updateMap := make(map[string]map[string]interface{})
	var err error

	updateMap["curseforge"], err = cfUpdater{
		ProjectID: modInfo.ID,
		FileID:    fileInfo.ID,
		// TODO: determine update channel
		ReleaseChannel: "beta",
	}.ToMap()
	if err != nil {
		return err
	}

	modMeta := core.Mod{
		Name:     modInfo.Name,
		FileName: fileInfo.FileName,
		Side:     core.UniversalSide,
		Download: core.ModDownload{
			URL: fileInfo.DownloadURL,
			// TODO: murmur2 hashing may be unstable in curse api, calculate the hash manually?
			// TODO: check if the hash is invalid (e.g. 0)
			HashFormat: "murmur2",
			Hash:       strconv.Itoa(fileInfo.Fingerprint),
		},
		Update: updateMap,
	}
	path := modMeta.SetMetaName(modInfo.Slug, flags)

	// If the file already exists, this will overwrite it!!!
	// TODO: Should this be improved?
	// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
	// VCS anyway.

	format, hash, err := modMeta.Write()
	if err != nil {
		return err
	}

	// TODO: send written data directly to index, instead of write+read?
	return index.RefreshFileWithHash(path, format, hash, true)
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

	done, modID, fileID, err := getFileIDsFromString(mod)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if !done {
		done, modID, err = getModIDFromString(mod)
		// Handle error later (e.g. lowercase to search instead of as a slug)
	}

	modInfoObtained := false
	var modInfoData modInfo

	if !done {
		modArgs := append([]string{mod}, modArgsTail...)
		searchTerm := strings.Join(modArgs, " ")
		// TODO: Curse search
		// TODO: how to do interactive choices? automatically assume version? ask mod from list? choose first?
		results, err := getSearch(searchTerm)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if len(results) == 0 {
			return cli.NewExitError(errors.New("no mods found"), 1)
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
				modInfoData, ok := menuRes[0].Value.(modInfo)
				if !ok {
					return errors.New("Error converting interface from wmenu")
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
			err = errors.New("Mod not found")
		}
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("ids: %d %d %v\n", modID, fileID, done)

	// TODO: get FileID if it isn't there

	fmt.Println(mcVersion)
	if !modInfoObtained {
		modInfoData, err = getModInfo(modID)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	if fileID == 0 {
		fmt.Println("WIP: get an actual file ID!!!")
		return nil
	}

	fileInfo, err := getFileInfo(modID, fileID)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	err = createModFile(flags, modInfoData, fileInfo, &index)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	err = index.Write()
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Mod \"%s\" successfully installed!\n", modInfoData.Name)

	return nil
}

type cfUpdateParser struct{}

func (u cfUpdateParser) ParseUpdate(updateUnparsed map[string]interface{}) (core.Updater, error) {
	var updater cfUpdater
	err := mapstructure.Decode(updateUnparsed, &updater)
	return updater, err
}

type cfUpdater struct {
	ProjectID      int    `mapstructure:"project-id"`
	FileID         int    `mapstructure:"file-id"`
	ReleaseChannel string `mapstructure:"release-channel"`
}

func (u cfUpdater) DoUpdate(mod core.Mod) (bool, error) {
	// TODO: implement updating
	return false, nil
}

func (u cfUpdater) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}
