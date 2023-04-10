package migrate

import (
	"github.com/packwiz/packwiz/cmd"
	"github.com/spf13/cobra"
)

// migrateCmd represents the base command when called without any subcommands
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate your Minecraft and loader versions to newer versions.",
}

func init() {
	cmd.Add(migrateCmd)
}
