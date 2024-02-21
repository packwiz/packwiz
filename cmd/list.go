package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all the mods in the modpack",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {

		// Load pack
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Load index
		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Load mods
		mods, err := index.LoadAllMods()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Filter mods by side
		if viper.IsSet("list.side") {
			side := viper.GetString("list.side")
			if side != core.UniversalSide && side != core.ServerSide && side != core.ClientSide {
				fmt.Printf("Invalid side %q, must be one of client, server, or both (default)\n", side)
				os.Exit(1)
			}

			i := 0
			for _, mod := range mods {
				if mod.Side == side || mod.Side == core.EmptySide || mod.Side == core.UniversalSide || side == core.UniversalSide {
					mods[i] = mod
					i++
				}
			}
			mods = mods[:i]
		}

		sort.Slice(mods, func(i, j int) bool {
			return strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name)
		})

		// Print mods
		if viper.GetBool("list.version") {
			for _, mod := range mods {
				fmt.Printf("%s (%s)\n", mod.Name, mod.FileName)
			}
		} else {
			for _, mod := range mods {
				fmt.Println(mod.Name)
			}
		}

		// Print mods in a Markdown table to a file
		if viper.IsSet("list.file") {
			// Get filename from argument, if any
			filename := viper.GetString("list.file")

			// Create file
			file, err := os.Create(filename)
			if err != nil {
				fmt.Println("Error creating file:", err)
				os.Exit(1)
			}
			defer file.Close()

			// Initialize max lengths
			maxNameLen, maxFileNameLen, maxSideLen := 0, 0, 0

			// Find max lengths
			for _, mod := range mods {
				if len(mod.Name) > maxNameLen {
					maxNameLen = len(mod.Name)
				}
				if len(mod.FileName) > maxFileNameLen {
					maxFileNameLen = len(mod.FileName)
				}
				if len(mod.Side) > maxSideLen {
					maxSideLen = len(mod.Side)
				}
			}

			// Write header
			fmt.Fprintf(file, "| %-*s | %-*s | %-*s |\n", maxNameLen, "Name", maxFileNameLen, "File Name", maxSideLen, "Side")
			fmt.Fprintf(file, "| %-*s | %-*s | %-*s |\n", maxNameLen, strings.Repeat("-", maxNameLen), maxFileNameLen, strings.Repeat("-", maxFileNameLen), maxSideLen, strings.Repeat("-", maxSideLen))

			// Write mods
			for _, mod := range mods {
				fmt.Fprintf(file, "| %-*s | %-*s | %-*s |\n", maxNameLen, mod.Name, maxFileNameLen, mod.FileName, maxSideLen, mod.Side)
			}

			// Print success message
			fmt.Println("Mod list written to", filename)
		} else {
			for _, mod := range mods {
				fmt.Println(mod.Name)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolP("version", "v", false, "Print name and version")
	_ = viper.BindPFlag("list.version", listCmd.Flags().Lookup("version"))
	listCmd.Flags().StringP("side", "s", "", "Filter mods by side (e.g., client or server)")
	_ = viper.BindPFlag("list.side", listCmd.Flags().Lookup("side"))
	listCmd.Flags().StringP("file", "f", "", "Print mods as a table to a Markdown file")
	_ = viper.BindPFlag("list.file", listCmd.Flags().Lookup("file"))

}
