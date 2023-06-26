package migrate

import (
	"fmt"
	"os"

	"github.com/packwiz/packwiz/core"
	"github.com/packwiz/packwiz/curseforge"
	"github.com/packwiz/packwiz/modrinth"
	"github.com/spf13/cobra"
)

var packFileCommand = &cobra.Command{
	Use:   "pack-file [version|latest]",
	Short: "Migrate your pack-file version to a newer version.",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
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

		// Auto-migrate versions
		if pack.PackFormat == "packwiz:1.0.0" {
			fmt.Println("Automatically migrating pack to packwiz:1.1.0 format...")
			pack.PackFormat = "packwiz:1.1.0"
		}
		if pack.PackFormat == "packwiz:1.1.0" {
			fmt.Println("Automatically migrating pack to packwiz:1.2.0 format...")
			pack.PackFormat = "packwiz:1.2.0"
			core.Pack.Write(pack)

			mods, err := index.LoadAllMods()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			for _, mod := range mods {

				// Reinstall modrinth mods
				_, ok := mod.Update["modrinth"]
				if ok {
					versionId, ok := mod.Update["modrinth"]["version"]
					if ok {
						if str, ok := versionId.(string); ok {
							modrinth.InstallVersionById(string(str), pack, &index)
						}
					}
				}

				// Reinstall curseforge mods
				// TODO change hacky way to reinstall
				_, ok = mod.Update["curseforge"]
				if ok {
					fileId, ok := mod.Update["curseforge"]["file-id"]
					if ok {
						projectId, ok := mod.Update["curseforge"]["project-id"]
						if ok {
							curseforgeCmd := curseforge.GetInstallCmd()
							curseforgeCmd.Flags().Set("addon-id", fmt.Sprintf("%d", projectId))
							curseforgeCmd.Flags().Set("file-id", fmt.Sprintf("%d", fileId))
							curseforgeCmd.Run(cmd, []string{})
						}
					}
				}
			}
		}
	},
}

func init() {
	migrateCmd.AddCommand(packFileCommand)
}
