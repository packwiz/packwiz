package curseforge

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/comp500/packwiz/core"
	"github.com/urfave/cli"
	"gopkg.in/dixonwille/wmenu.v4"
)

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

	var done bool
	var modID, fileID int
	// If modArgsTail has anything, go straight to searching - URLs/Slugs should not have spaces!
	if len(modArgsTail) == 0 {
		done, modID, fileID, err = getFileIDsFromString(mod)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if !done {
			done, modID, err = getModIDFromString(mod)
			// Ignore error, go to search instead (e.g. lowercase to search instead of as a slug)
			if err != nil {
				done = false
			}
		}
	}

	modInfoObtained := false
	var modInfoData modInfo

	if !done {
		fmt.Println("Searching CurseForge...")
		modArgs := append([]string{mod}, modArgsTail...)
		searchTerm := strings.Join(modArgs, " ")
		// TODO: Curse search
		// TODO: how to do interactive choices? automatically assume version? ask mod from list? choose first?
		results, err := getSearch(searchTerm, mcVersion)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if len(results) == 0 {
			return cli.NewExitError("No mods found!", 1)
		} else if len(results) == 1 {
			modInfoData = results[0]
			modID = modInfoData.ID
			modInfoObtained = true
			done = true
		} else {
			// Find the closest value to the search term
			sort.Slice(results, func(i, j int) bool {
				return levenshtein.ComputeDistance(searchTerm, results[i].Name) < levenshtein.ComputeDistance(searchTerm, results[j].Name)
			})

			menu := wmenu.NewMenu("Choose a number:")

			for i, v := range results {
				menu.Option(v.Name, v, i == 0, nil)
			}
			menu.Option("Cancel", nil, false, nil)

			menu.Action(func(menuRes []wmenu.Opt) error {
				if len(menuRes) != 1 || menuRes[0].Value == nil {
					fmt.Println("Cancelled!")
					return nil
				}

				// Why is variable shadowing a thing!!!!
				var ok bool
				modInfoData, ok = menuRes[0].Value.(modInfo)
				if !ok {
					return cli.NewExitError("Error converting interface from wmenu", 1)
				}
				modID = modInfoData.ID
				modInfoObtained = true
				done = true
				return nil
			})
			err = menu.Run()
			if err != nil {
				return cli.NewExitError(err, 1)
			}

			if !done {
				return nil
			}
		}
	}

	if !done {
		if err == nil {
			return cli.NewExitError("No mods found!", 1)
		}
		return cli.NewExitError(err, 1)
	}

	if !modInfoObtained {
		modInfoData, err = getModInfo(modID)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	fileInfoObtained := false
	var fileInfoData modFileInfo
	if fileID == 0 {
		// TODO: how do we decide which version to use?
		for _, v := range modInfoData.GameVersionLatestFiles {
			// Choose "newest" version by largest ID
			if v.GameVersion == mcVersion && v.ID > fileID {
				fileID = v.ID
			}
		}

		if fileID == 0 {
			return cli.NewExitError("No files available for current Minecraft version!", 1)
		}

		// The API also provides some files inline, because that's efficient!
		for _, v := range modInfoData.LatestFiles {
			if v.ID == fileID {
				fileInfoObtained = true
				fileInfoData = v
			}
		}
	}

	if !fileInfoObtained {
		fileInfoData, err = getFileInfo(modID, fileID)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	err = createModFile(flags, modInfoData, fileInfoData, &index)
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

	fmt.Printf("Mod \"%s\" successfully installed! (%s)\n", modInfoData.Name, fileInfoData.FileName)

	return nil
}
