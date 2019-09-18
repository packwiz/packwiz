package utils

import (
	"github.com/comp500/packwiz/cmd"
	"github.com/spf13/cobra"
)

// utilsCmd represents the base command when called without any subcommands
var utilsCmd = &cobra.Command{
	Use:   "utils",
	Short: "Utilities for managing packwiz itself",
}

func init() {
	cmd.Add(utilsCmd)
}
