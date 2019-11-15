package utils

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
)

// markdownCmd represents the markdown command
var markdownCmd = &cobra.Command{
	Use:     "markdown",
	Short:   "Generate markdown documentation (that you might be reading right now!!)",
	Aliases: []string{"md"},
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		outDir := viper.GetString("utils.markdown.dir")
		err := os.MkdirAll(outDir, os.ModePerm)
		if err != nil {
			fmt.Printf("Error creating directory: %s\n", err)
			os.Exit(1)
		}
		err = doc.GenMarkdownTree(cmd.Root(), outDir)
		if err != nil {
			fmt.Printf("Error generating markdown: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("Generated markdown successfully!")
	},
}

func init() {
	utilsCmd.AddCommand(markdownCmd)

	markdownCmd.Flags().String("dir", ".", "The destination directory to save docs in")
	_ = viper.BindPFlag("utils.markdown.dir", markdownCmd.Flags().Lookup("dir"))
}
