package cmd

import (
	"fmt"
	"os"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:     "remove",
	Short:   "Remove a mod from the modpack",
	Aliases: []string{"delete", "uninstall", "rm"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args[0]) == 0 {
			fmt.Println("You must specify a mod.")
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
		resolvedMod, ok := index.FindMod(args[0])
		if !ok {
			fmt.Println("You don't have this mod installed.")
			os.Exit(1)
		}
		err = os.Remove(resolvedMod)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Removing mod from index...")
		err = index.RemoveFile(resolvedMod)
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

		fmt.Printf("Mod %s removed successfully!\n", args[0])
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
