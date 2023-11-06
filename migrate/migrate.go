package migrate

import (
	"packwiz/cmd"
	"github.com/spf13/cobra"
)

// migrateCmd represents the base command when called without any subcommands
var migrateCmd = &cobra.Command{
	Use:   "migrate [minecraft|loader]",
	Short: "Migrate your Minecraft and loader versions to newer versions.",
}

func init() {
	cmd.Add(migrateCmd)
}
