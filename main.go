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
			return cmdDelete(core.FlagsFromContext(c), c.Args().Get(0))
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

func cmdDelete(flags core.Flags, mod string) error {
	if len(mod) == 0 {
		return cli.NewExitError("You must specify a mod.", 1)
	}
	resolvedMod := core.ResolveMod(mod, flags)
	err := os.Remove(resolvedMod)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Println("Loading modpack...")
	pack, err := core.LoadPack(flags)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	index, err := pack.LoadIndex()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Println("Removing mod from index...")
	err = index.RemoveFile(resolvedMod)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = index.Write()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = pack.UpdateIndexHash()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = pack.Write()
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Mod %s removed successfully!", mod)
	return nil
}

func cmdRefresh(flags core.Flags) error {
	fmt.Println("Loading modpack...")
	pack, err := core.LoadPack(flags)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	index, err := pack.LoadIndex()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Println("Refreshing index...")
	err = index.Refresh()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = index.Write()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = pack.UpdateIndexHash()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = pack.Write()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Println("Index refreshed!")
	return nil
}

