package curseforge

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/comp500/packwiz/core"
	"github.com/mitchellh/mapstructure"
	"github.com/urfave/cli"
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

func createModFile(flags core.Flags, modID int, fileID int, modInfo modInfo) error {
	fileInfo, err := getFileInfo(modID, fileID)

	updateMap := make(map[string]map[string]interface{})

	updateMap["curseforge"], err = cfUpdater{
		ProjectID: modID,
		FileID:    fileID,
		// TODO: determine update channel
		ReleaseChannel: "release",
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
			HashFormat: "murmur2",
			Hash:       strconv.Itoa(fileInfo.Fingerprint),
		},
		Update: updateMap,
	}
	modMeta.SetMetaName(modInfo.Slug, flags)

	fmt.Printf("%#v\n", modMeta)

	// TODO: what to do if it already exists?
	return modMeta.Write()
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

	if !done {
		modArgs := append([]string{mod}, modArgsTail...)
		searchTerm := strings.Join(modArgs, " ")
		// TODO: CurseMeta search
		// TODO: how to do interactive choices? automatically assume version? ask mod from list? choose first?
		fmt.Println(searchTerm)
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
	modInfo, err := getModInfo(modID)
	fmt.Println(err)
	fmt.Println(modInfo)
	_ = index

	if fileID == 0 {
		return nil
	}

	return createModFile(flags, modID, fileID, modInfo)
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
	return false, nil
}

func (u cfUpdater) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}

