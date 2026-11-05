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

		// Filter mods by pin status
		showPinned := viper.GetBool("list.pinned")
		showUnpinned := viper.GetBool("list.unpinned")

		if showPinned && showUnpinned {
			fmt.Println("Cannot specify both --pinned and --unpinned flags")
			os.Exit(1)
		}

		if showPinned || showUnpinned {
			i := 0
			for _, mod := range mods {
				if (showPinned && mod.Pin) || (showUnpinned && !mod.Pin) {
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
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolP("version", "v", false, "Print name and version")
	_ = viper.BindPFlag("list.version", listCmd.Flags().Lookup("version"))
	listCmd.Flags().StringP("side", "s", "", "Filter mods by side (e.g., client or server)")
	_ = viper.BindPFlag("list.side", listCmd.Flags().Lookup("side"))
	listCmd.Flags().Bool("pinned", false, "Show only pinned mods")
	_ = viper.BindPFlag("list.pinned", listCmd.Flags().Lookup("pinned"))
	listCmd.Flags().Bool("unpinned", false, "Show only unpinned mods")
	_ = viper.BindPFlag("list.unpinned", listCmd.Flags().Lookup("unpinned"))

}
