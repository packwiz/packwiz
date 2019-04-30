package curseforge
import (
	"fmt"
	"regexp"
	"strconv"

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
				return cmdInstall(core.FlagsFromContext(c), c.Args().Get(0))
			},
		}, {
			Name:  "import",
			Usage: "Import an installed curseforge modpack",
			Action: func(c *cli.Context) error {
				fmt.Println("Not implemented yet!")
				return nil
			},
		}},
	})
	core.UpdateParsers["curseforge"] = cfUpdateParser{}
}

var fileIDRegexes = [...]*regexp.Regexp{
	regexp.MustCompile("https?:\\/\\/minecraft\\.curseforge\\.com\\/projects\\/(.+)\\/files\\/(\\d+)"),
	regexp.MustCompile("https?:\\/\\/(?:www\\.)?curseforge\\.com\\/minecraft\\/mc-mods\\/(.+)\\/download\\/(\\d+)"),
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
	regexp.MustCompile("https?:\\/\\/minecraft\\.curseforge\\.com\\/projects\\/([^\\/]+)"),
	regexp.MustCompile("https?:\\/\\/(?:www\\.)?curseforge\\.com\\/minecraft\\/mc-mods\\/([^\\/]+)"),
	// Exact slug matcher
	regexp.MustCompile("[a-z][\\da-z\\-]{0,127}"),
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

func cmdInstall(flags core.Flags, mod string) error {
	if len(mod) == 0 {
		return cli.NewExitError("You must specify a mod.", 1)
	}
	//fmt.Println("Not implemented yet!")

	done, modID, fileID, err := getFileIDsFromString(mod)
	if err != nil {
		fmt.Println(err)
	}

	if !done {
		done, modID, err = getModIDFromString(mod)
		if err != nil {
			fmt.Println(err)
		}
	}

	// TODO: fallback to CurseMeta search
	// TODO: how to do interactive choices? automatically assume version? ask mod from list? choose first?

	fmt.Printf("ids: %d %d %v", modID, fileID, done)
	return nil
}

type cfUpdateParser struct{}

func (u cfUpdateParser) ParseUpdate(updateUnparsed interface{}) (core.Updater, error) {
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

