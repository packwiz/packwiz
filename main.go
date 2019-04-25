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
		Name:    "remove",
		Aliases: []string{"delete", "uninstall"},
		Usage:   "Remove a mod from the modpack",
		Action: func(c *cli.Context) error {
			return cmdDelete(core.FlagsFromContext(c))
		},
	}, cli.Command{
		Name:    "update",
		Aliases: []string{"upgrade"},
		Usage:   "Update a mod (or all mods) in the modpack",
		Action: func(c *cli.Context) error {
			// TODO: implement
			return nil
		},
	}, cli.Command{
		Name:  "refresh",
		Usage: "Refresh the index file",
		Action: func(c *cli.Context) error {
			return cmdRefresh(core.FlagsFromContext(c))
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

func cmdDelete(flags core.Flags) error {
	mod := "demagnetize"
	err := os.Remove(core.ResolveMod(mod, flags))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Printf("Mod %s removed successfully!", mod)
	// TODO: update index
	return nil
}

func cmdRefresh(flags core.Flags) error {
	index, err := core.LoadIndex(flags)
	if err != nil {
		return err
	}
	err = index.Refresh()
	if err != nil {
		return err
	}
	err = index.Write()
	if err != nil {
		return err
	}
	return nil
}

