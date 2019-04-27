package curseforge
import (
	"fmt"

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
func cmdInstall(flags core.Flags, mod string) error {
	if len(mod) == 0 {
		return cli.NewExitError("You must specify a mod.", 1)
	}
	fmt.Println("Not implemented yet!")
	return nil
}

type cfUpdateParser struct{}

func (u cfUpdateParser) ParseUpdate(updateUnparsed interface{}) (core.Updater, error) {
	var updater cfUpdater
	err := mapstructure.Decode(updateUnparsed, &updater)
	return updater, err
}

type cfUpdater struct {
	ProjectID int `mapstructure:"project-id"`
}

func (u cfUpdater) DoUpdate(mod core.Mod) (bool, error) {
	return false, nil
}

