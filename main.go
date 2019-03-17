package main
import (
	"fmt"
	"log"
	"os"

	"github.com/comp500/packwiz/core"
	"github.com/urfave/cli"

	// Modules of packwiz
	_ "github.com/comp500/packwiz/curseforge"
)

func init() {
	core.Commands = append(core.Commands, cli.Command{
		Name:    "delete",
		Aliases: []string{"remove"},
		Usage:   "Delete a mod from the modpack",
		Action: func(c *cli.Context) error {
			cmdDelete(core.FlagsFromContext(c))
			return nil
		},
	}, cli.Command{
		Name:    "delet",
		Aliases: []string{"remov"},
		Usage:   "Delete a mod from the modpack",
		Action: func(c *cli.Context) error {
			cmdDelete(core.FlagsFromContext(c))
			return nil
		},
	})
}

func main() {
	app := cli.NewApp()
	app.Commands = core.Commands
	app.Flags = core.CLIFlags[:]
	app.HideVersion = true
	app.Usage = "A command line tool for creating Minecraft modpacks."

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func cmdDelete(flags core.Flags) {
	mod := "demagnetize"
	err := os.Remove(core.ResolveMod(mod, flags))
	if err != nil {
		fmt.Printf("Error removing mod: %s", err)
	} else {
		fmt.Printf("Mod %s removed successfully!", mod)
	}
	// TODO: update index
}

