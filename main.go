package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"bufio"

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
	// TODO: --check flag?
	// TODO: specify multiple mods to update at once?

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

		updaterMap := make(map[string][]core.Mod)
		fmt.Println("Reading mod files...")
		for _, v := range index.GetAllMods() {
			modData, err := core.LoadMod(v)
			if err != nil {
				fmt.Printf("Error reading mod file: %s", err.Error())
				continue
			}

			updaterFound := false
			for k := range modData.Update {
				slice, ok := updaterMap[k]
				if !ok {
					_, ok = core.Updaters[k]
					if !ok {
						continue
					}
					slice = []core.Mod{}
				}
				updaterFound = true
				updaterMap[k] = append(slice, modData)
			}
			if !updaterFound {
				fmt.Printf("A supported update system for \"%s\" cannot be found.", modData.Name)
			}
		}

		fmt.Println("Checking for updates...")
		updatesFound := false
		updaterPointerMap := make(map[string][]*core.Mod)
		updaterCachedStateMap := make(map[string][]interface{})
		for k, v := range updaterMap {
			checks, err := core.Updaters[k].CheckUpdate(v, mcVersion)
			if err != nil {
				// TODO: do we return err code 1?
				fmt.Println(err.Error())
				continue
			}
			for i, check := range checks {
				if check.Error != nil {
					// TODO: do we return err code 1?
					// TODO: better error message?
					fmt.Println(check.Error.Error())
					continue
				}
				if check.UpdateAvailable {
					if !updatesFound {
						fmt.Println("Updates found:")
						updatesFound = true
					}
					fmt.Printf("%s: %s\n", v[i].Name, check.UpdateString)
					updaterPointerMap[k] = append(updaterPointerMap[k], &v[i])
					updaterCachedStateMap[k] = append(updaterCachedStateMap[k], check.CachedState)
				}
			}
		}

		if !updatesFound {
			fmt.Println("All mods are up to date!")
			return nil
		}

		fmt.Print("Do you want to update? [Y/n]: ")
		answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		ansNormal := strings.ToLower(strings.TrimSpace(answer))
		if len(ansNormal) > 0 && ansNormal[0] == 'n' {
			fmt.Println("Cancelled!")
			return nil
		}

		for k, v := range updaterPointerMap {
			err := core.Updaters[k].DoUpdate(v, updaterCachedStateMap[k])
			if err != nil {
				// TODO: do we return err code 1?
				fmt.Println(err.Error())
				continue
			}
			for _, modData := range v {
				format, hash, err := modData.Write()
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				err = index.RefreshFileWithHash(modData.GetFilePath(), format, hash, true)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
			}
		}
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
			// TODO: use file name instead of Name when len(Name) == 0 in all places?
			return cli.NewExitError("A supported update system for \""+modData.Name+"\" cannot be found.", 1)
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
