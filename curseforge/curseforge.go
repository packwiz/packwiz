package curseforge

import (
	"fmt"

	"github.com/comp500/packwiz/core"
	"github.com/urfave/cli"
)

func init() {
	core.Commands = append(core.Commands, cli.Command{
		Name:  "curseforge",
		Usage: "Manage curseforge-based mods",
		Action: func(c *cli.Context) error {
			fmt.Println("Not implemented yet!")
			return nil
		},
	})
}
