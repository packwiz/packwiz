package cmd

import (
	"fmt"
	"github.com/packwiz/packwiz/cmdshared"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:     "update [mod]",
	Short:   "Update a mod (or all mods) in the modpack",
	Aliases: []string{"upgrade"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: --check flag?
		// TODO: specify multiple mods to update at once?

		fmt.Println("Loading modpack...")
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		mcVersion, err := pack.GetMCVersion()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var singleUpdatedName string
		if viper.GetBool("update.all") {
			updaterMap := make(map[string][]core.Mod)
			fmt.Println("Reading mod files...")
			for _, v := range index.GetAllMods() {
				modData, err := core.LoadMod(v)
				if err != nil {
					fmt.Printf("Error reading mod file: %s\n", err.Error())
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
					fmt.Printf("A supported update system for \"%s\" cannot be found.\n", modData.Name)
				}
			}

			fmt.Println("Checking for updates...")
			updatesFound := false
			updaterPointerMap := make(map[string][]*core.Mod)
			updaterCachedStateMap := make(map[string][]interface{})
			for k, v := range updaterMap {
				checks, err := core.Updaters[k].CheckUpdate(v, mcVersion, pack)
				if err != nil {
					// TODO: do we return err code 1?
					fmt.Printf("Failed to check updates for %s: %s\n", k, err.Error())
					continue
				}
				for i, check := range checks {
					if check.Error != nil {
						// TODO: do we return err code 1?
						fmt.Printf("Failed to check updates for %s: %s\n", v[i].Name, check.Error.Error())
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
				return
			}

			if !cmdshared.PromptYesNo("Do you want to update? [Y/n]: ") {
				fmt.Println("Cancelled!")
				return
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
			if len(args) < 1 || len(args[0]) == 0 {
				fmt.Println("Must specify a valid mod, or use the --all flag!")
				os.Exit(1)
			}
			modPath, ok := index.FindMod(args[0])
			if !ok {
				fmt.Println("You don't have this mod installed.")
				os.Exit(1)
			}
			modData, err := core.LoadMod(modPath)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			singleUpdatedName = modData.Name
			updaterFound := false
			for k := range modData.Update {
				updater, ok := core.Updaters[k]
				if !ok {
					continue
				}
				updaterFound = true

				check, err := updater.CheckUpdate([]core.Mod{modData}, mcVersion, pack)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				if len(check) != 1 {
					fmt.Println("Invalid update check response")
					os.Exit(1)
				}

				if check[0].UpdateAvailable {
					fmt.Printf("Update available: %s\n", check[0].UpdateString)

					err = updater.DoUpdate([]*core.Mod{&modData}, []interface{}{check[0].CachedState})
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}

					format, hash, err := modData.Write()
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					err = index.RefreshFileWithHash(modPath, format, hash, true)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
				} else {
					fmt.Printf("\"%s\" is already up to date!\n", modData.Name)
					return
				}

				break
			}
			if !updaterFound {
				// TODO: use file name instead of Name when len(Name) == 0 in all places?
				fmt.Println("A supported update system for \"" + modData.Name + "\" cannot be found.")
				os.Exit(1)
			}
		}

		err = index.Write()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = pack.Write()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if viper.GetBool("update.all") {
			fmt.Println("Mods updated!")
		} else {
			fmt.Printf("\"%s\" updated!\n", singleUpdatedName)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().BoolP("all", "a", false, "Update all mods")
	_ = viper.BindPFlag("update.all", updateCmd.Flags().Lookup("all"))
}
