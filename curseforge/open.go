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
	Use: "open",
	// TODO: change semantics to "project" rather than "mod", as this supports texture packs and misc content as well?
	Short:   "Open the project page for a curseforge mod in your browser",
	Aliases: []string{"doc"},
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
			// TODO: should this auto-refresh???????
			fmt.Println("You don't have this mod installed.")
			os.Exit(1)
		}
		modData, err := core.LoadMod(resolvedMod)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		updateData, ok := modData.GetParsedUpdateData("curseforge")
		if !ok {
			fmt.Println("This mod doesn't seem to be a curseforge mod!")
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
