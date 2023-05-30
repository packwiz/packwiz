package url

import (
	"github.com/packwiz/packwiz/cmd"
	"github.com/spf13/cobra"
)

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "Add external files from a direct download link, for sites that are not directly supported by packwiz",
}

func init() {
	cmd.Add(urlCmd)
}
