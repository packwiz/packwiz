package curseforge

import (
	"archive/zip"
	"bufio"
	"fmt"
	"github.com/packwiz/packwiz/cmdshared"
	"github.com/packwiz/packwiz/core"
	"github.com/packwiz/packwiz/curseforge/packinterop"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strconv"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the current modpack into a .zip for curseforge",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		side := viper.GetString("curseforge.export.side")
		if len(side) == 0 || (side != core.UniversalSide && side != core.ServerSide && side != core.ClientSide) {
			fmt.Println("Invalid side!")
			os.Exit(1)
		}

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
		// Do a refresh to ensure files are up to date
		err = index.Refresh()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
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

		// TODO: should index just expose indexPath itself, through a function?
		indexPath := filepath.Join(filepath.Dir(viper.GetString("pack-file")), filepath.FromSlash(pack.Index.File))

		fmt.Println("Reading external files...")
		mods, err := index.LoadAllMods()
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			os.Exit(1)
		}
		i := 0
		// Filter mods by side
		// TODO: opt-in optional disabled filtering?
		for _, mod := range mods {
			if mod.Side == side || mod.Side == core.EmptySide || mod.Side == core.UniversalSide || side == core.UniversalSide {
				mods[i] = mod
				i++
			}
		}
		mods = mods[:i]

		var exportData cfExportData
		exportDataUnparsed, ok := pack.Export["curseforge"]
		if ok {
			exportData, err = parseExportData(exportDataUnparsed)
			if err != nil {
				fmt.Printf("Failed to parse export metadata: %s\n", err.Error())
				os.Exit(1)
			}
		}

		fileName := viper.GetString("curseforge.export.output")
		if fileName == "" {
			fileName = pack.GetPackName() + ".zip"
		}

		expFile, err := os.Create(fileName)
		if err != nil {
			fmt.Printf("Failed to create zip: %s\n", err.Error())
			os.Exit(1)
		}
		exp := zip.NewWriter(expFile)

		// Add an overrides folder even if there are no files to go in it
		_, err = exp.Create("overrides/")
		if err != nil {
			fmt.Printf("Failed to add overrides folder: %s\n", err.Error())
			os.Exit(1)
		}

		cfFileRefs := make([]packinterop.AddonFileReference, 0, len(mods))
		nonCfMods := make([]*core.Mod, 0)
		for _, mod := range mods {
			projectRaw, ok := mod.GetParsedUpdateData("curseforge")
			// If the mod has curseforge metadata, add it to cfFileRefs
			if ok {
				p := projectRaw.(cfUpdateData)
				cfFileRefs = append(cfFileRefs, packinterop.AddonFileReference{
					ProjectID:        p.ProjectID,
					FileID:           p.FileID,
					OptionalDisabled: mod.Option != nil && mod.Option.Optional && !mod.Option.Default,
				})
			} else {
				nonCfMods = append(nonCfMods, mod)
			}
		}

		// Download external files and save directly into the zip
		if len(nonCfMods) > 0 {
			fmt.Printf("Retrieving %v external files to store in the modpack zip...\n", len(nonCfMods))
			cmdshared.PrintDisclaimer(true)

			session, err := core.CreateDownloadSession(nonCfMods, []string{})
			if err != nil {
				fmt.Printf("Error retrieving external files: %v\n", err)
				os.Exit(1)
			}

			cmdshared.ListManualDownloads(session)

			for dl := range session.StartDownloads() {
				_ = cmdshared.AddToZip(dl, exp, "overrides", indexPath)
			}

			err = session.SaveIndex()
			if err != nil {
				fmt.Printf("Error saving cache index: %v\n", err)
				os.Exit(1)
			}
		}

		manifestFile, err := exp.Create("manifest.json")
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error creating manifest: " + err.Error())
			os.Exit(1)
		}

		err = packinterop.WriteManifestFromPack(pack, cfFileRefs, exportData.ProjectID, manifestFile)
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error writing manifest: " + err.Error())
			os.Exit(1)
		}

		err = createModlist(exp, mods)
		if err != nil {
			_ = exp.Close()
			_ = expFile.Close()
			fmt.Println("Error creating mod list: " + err.Error())
			os.Exit(1)
		}

		i = 0
		for _, v := range index.Files {
			if !v.MetaFile {
				// Save all non-metadata files into the zip
				path, err := filepath.Rel(filepath.Dir(indexPath), index.GetFilePath(v))
				if err != nil {
					fmt.Printf("Error resolving file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				file, err := exp.Create(filepath.ToSlash(filepath.Join("overrides", path)))
				if err != nil {
					fmt.Printf("Error creating file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				err = index.SaveFile(v, file)
				if err != nil {
					fmt.Printf("Error copying file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				i++
			}
		}

		err = exp.Close()
		if err != nil {
			fmt.Println("Error writing export file: " + err.Error())
			os.Exit(1)
		}
		err = expFile.Close()
		if err != nil {
			fmt.Println("Error writing export file: " + err.Error())
			os.Exit(1)
		}

		fmt.Println("Modpack exported to " + fileName)
	},
}

func createModlist(zw *zip.Writer, mods []*core.Mod) error {
	modlistFile, err := zw.Create("modlist.html")
	if err != nil {
		return err
	}

	w := bufio.NewWriter(modlistFile)

	_, err = w.WriteString("<ul>\r\n")
	if err != nil {
		return err
	}
	for _, mod := range mods {
		projectRaw, ok := mod.GetParsedUpdateData("curseforge")
		if !ok {
			// TODO: read homepage URL or something similar?
			// TODO: how to handle mods that don't have metadata???
			_, err = w.WriteString("<li>" + mod.Name + "</li>\r\n")
			if err != nil {
				return err
			}
			continue
		}
		project := projectRaw.(cfUpdateData)
		_, err = w.WriteString("<li><a href=\"https://www.curseforge.com/projects/" + strconv.FormatUint(uint64(project.ProjectID), 10) + "\">" + mod.Name + "</a></li>\r\n")
		if err != nil {
			return err
		}
	}
	_, err = w.WriteString("</ul>\r\n")
	if err != nil {
		return err
	}
	return w.Flush()
}

func init() {
	curseforgeCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringP("side", "s", "client", "The side to export mods with")
	_ = viper.BindPFlag("curseforge.export.side", exportCmd.Flags().Lookup("side"))
	exportCmd.Flags().StringP("output", "o", "", "The file to export the modpack to")
	_ = viper.BindPFlag("curseforge.export.output", exportCmd.Flags().Lookup("output"))
}
