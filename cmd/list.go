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
		for _, mod := range mods {
			var output string
			if viper.GetBool("list.version") {
				output = fmt.Sprintf("%s (%s)", mod.Name, mod.FileName)
			} else {
				output = mod.Name
			}

			var provider string
			if strings.Contains(mod.Download.URL, "cdn.modrinth.com") {
				provider = "Modrinth"
			} else if mod.Download.Mode == "metadata:curseforge" {
				provider = "CurseForge"
			} else {
				provider = "Unknown"
			}

			var slug = strings.FieldsFunc(mod.GetFilePath(), func(r rune) bool {
				return strings.ContainsRune("/.", r)
			})[1]

			if viper.GetBool("list.slug") {
				if viper.GetBool("list.provider") {
					fmt.Printf("%s: %s\n", provider, slug)
				} else {
					fmt.Printf("%s\n", slug)
				}
			} else if viper.GetBool("list.provider") {
				fmt.Printf("%s: %s\n", provider, output)
			} else {
				fmt.Println(output)
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
	listCmd.Flags().BoolP("slug", "g", false, "List as slugs (file ids)")
	_ = viper.BindPFlag("list.slug", listCmd.Flags().Lookup("slug"))
	listCmd.Flags().BoolP("provider", "p", false, "List with mod provider")
	_ = viper.BindPFlag("list.provider", listCmd.Flags().Lookup("provider"))
}
