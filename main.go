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
			return cmdUpdate(core.FlagsFromContext(c), c.Args().Get(0))
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
	fmt.Println("Loading modpack...")
	pack, err := core.LoadPack(flags)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	index, err := pack.LoadIndex()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	resolvedMod, ok := index.FindMod(mod)
	if !ok {
		return cli.NewExitError("You don't have this mod installed.", 1)
	}
	err = os.Remove(resolvedMod)
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

func cmdUpdate(flags core.Flags, mod string) error {
	fmt.Println("Loading modpack...")
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

	multiple := false
	var singleUpdatedName string
	if len(mod) == 0 || mod == "*" {
		multiple = true
		// TODO: implement
		return cli.NewExitError("Not implemented yet!", 1)
	} else {
		modPath, ok := index.FindMod(mod)
		if !ok {
			return cli.NewExitError("You don't have this mod installed.", 1)
		}
		modData, err := core.LoadMod(modPath)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		singleUpdatedName = modData.Name
		updaterFound := false
		for k := range modData.Update {
			updater, ok := core.Updaters[k]
			if !ok {
				continue
			}
			updaterFound = true

			check, err := updater.CheckUpdate([]core.Mod{modData}, mcVersion)
			if err != nil {
				return cli.NewExitError(err, 1)
			}
			if len(check) != 1 {
				return cli.NewExitError("Invalid update check response", 1)
			}

			if check[0].UpdateAvailable {
				fmt.Printf("Update available: %s\n", check[0].UpdateString)

				err = updater.DoUpdate([]*core.Mod{&modData}, []interface{}{check[0].CachedState})
				if err != nil {
					return cli.NewExitError(err, 1)
				}

				format, hash, err := modData.Write()
				if err != nil {
					return cli.NewExitError(err, 1)
				}
				err = index.RefreshFileWithHash(modPath, format, hash, true)
				if err != nil {
					return cli.NewExitError(err, 1)
				}
			} else {
				fmt.Printf("\"%s\" is already up to date!\n", modData.Name)
				return nil
			}

			break
		}
		if !updaterFound {
			return cli.NewExitError("A supported update system for this mod cannot be found.", 1)
		}
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
	if multiple {
		fmt.Println("Mods updated!")
	} else {
		fmt.Printf("\"%s\" updated!\n", singleUpdatedName)
	}
	return nil
}
