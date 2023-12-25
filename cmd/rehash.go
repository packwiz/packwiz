package cmd

import (
	"fmt"
	"os"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

// rehashCmd represents the rehash command
var rehashCmd = &cobra.Command{
	Use:   "rehash",
	Short: "Migrate all hashes to a specific function",
	Args:  cobra.ExactArgs(1),
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
		
		session, err := core.CreateDownloadSession(mods, []string{args[0]})
		if err != nil {
			fmt.Printf("Error retrieving external files: %v\n", err)
			os.Exit(1)
		}
		for dl := range session.StartDownloads() {
			if dl.Error != nil {
				fmt.Printf("Error retrieving %s: %v\n", dl.Mod.Name, dl.Error);
			}
			dl.Mod.Download.HashFormat = args[0]
			dl.Mod.Download.Hash = dl.Hashes[args[0]]
			_, _, err := dl.Mod.Write()
			if err != nil {
				fmt.Printf("Error saving mod %s: %v\n", dl.Mod.Name, err)
				os.Exit(1)
			}
			// TODO pass the hash to index instead of recomputing from scratch
		}
		
		err = index.Refresh()
		if err != nil {
			fmt.Printf("Error refreshing index: %v\n", err)
			os.Exit(1)
		}
		
		err = index.Write()
		if err != nil {
			fmt.Printf("Error writing index: %v\n", err)
			os.Exit(1)
		}
		
		err = pack.UpdateIndexHash()
		if err != nil {
			fmt.Printf("Error updating index hash: %v\n", err)
			os.Exit(1)
		}
		
		err = pack.Write()
		if err != nil {
			fmt.Printf("Error writing pack: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(rehashCmd)
}
