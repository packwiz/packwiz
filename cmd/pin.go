package cmd

import (
	"fmt"
	"os"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

func createPinRun(pinned bool) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
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
		modPath, ok := index.FindMod(args[0])
		if !ok {
			fmt.Println("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
			os.Exit(1)
		}
		modData, err := core.LoadMod(modPath)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		modData.Pin = pinned
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

		message := "pinned"
		if !pinned {
			message = "unpinned"
		}
		fmt.Printf("%s %s successfully!\n", args[0], message)
	}
}

// pinCmd represents the pin command
var pinCmd = &cobra.Command{
	Use:   "pin",
	Short: "Pin a mod to the current version",
	Args:  cobra.ExactArgs(1),
	Run:   createPinRun(true),
}

// unpinCmd represents the unpin command
var unpinCmd = &cobra.Command{
	Use:   "unpin",
	Short: "Unpin a mod from the current version",
	Args:  cobra.ExactArgs(1),
	Run:   createPinRun(false),
}

func init() {
	rootCmd.AddCommand(pinCmd)
	rootCmd.AddCommand(unpinCmd)
}
