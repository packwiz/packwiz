package migrate

import (
	"fmt"
	"github.com/spf13/cobra"
)

var loaderCommand = &cobra.Command{
	Use:   "loader",
	Short: "Migrate your loader versions to newer versions.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if args[0] == "latest" {
			fmt.Println("Updating to latest loader version")
		} else if args[0] == "recommended" {
			fmt.Println("Updating to recommended loader version")
		} else {
			fmt.Println("Updating to explicit loader version")
		}
	},
}

func init() {
	migrateCmd.AddCommand(loaderCommand)
}
