package curseforge

import (
	"fmt"
	"os"
	"strconv"

	"github.com/packwiz/packwiz/core"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

// openCmd represents the open command
var openCmd = &cobra.Command{
	Use:     "open [name]",
	Short:   "Open the project page for a CurseForge file in your browser",
	Aliases: []string{"doc"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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
			// TODO: should this auto-refresh?
			fmt.Println("Can't find this file; please ensure you have run packwiz refresh and use the name of the .pw.toml file (defaults to the project slug)")
			os.Exit(1)
		}
		modData, err := core.LoadMod(resolvedMod)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		updateData, ok := modData.GetParsedUpdateData("curseforge")
		if !ok {
			fmt.Println("Can't find CurseForge update metadata for this file")
			os.Exit(1)
		}
		cfUpdateData := updateData.(cfUpdateData)
		fmt.Println("Opening browser...")
		url := "https://www.curseforge.com/projects/" + strconv.FormatUint(uint64(cfUpdateData.ProjectID), 10)
		err = open.Start(url)
		if err != nil {
			fmt.Println("Opening page failed, direct link:")
			fmt.Println(url)
		}
	},
}

func init() {
	curseforgeCmd.AddCommand(openCmd)
}
