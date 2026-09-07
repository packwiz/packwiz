package cmd

import (
	"archive/zip"
	"fmt"
	"os"

	"github.com/packwiz/packwiz/cmdshared"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// downloadCmd represents the download command
var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download all mods into a zip file",
	Args:  cobra.NoArgs,
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

		// Do a refresh to ensure files are up to date
		err = index.Refresh()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = index.Write()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			fmt.Println(err)
			return
		}
		err = pack.Write()
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("Reading external files...")
		mods, err := index.LoadAllMods()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fileName := viper.GetString("download.output")
		if fileName == "" {
			fileName = pack.GetPackName() + ".zip"
		}
		expFile, err := os.Create(fileName)
		if err != nil {
			fmt.Printf("Failed to create zip: %s\n", err.Error())
			os.Exit(1)
		}
		exp := zip.NewWriter(expFile)

		session, err := core.CreateDownloadSession(mods, []string{"sha1", "sha512", "length-bytes"})
		if err != nil {
			fmt.Printf("Error retrieving external files: %v\n", err)
			os.Exit(1)
		}
		cmdshared.ListManualDownloads(session)
		for dl := range session.StartDownloads() {
			_ = cmdshared.AddToZip(dl, exp, "", &index)
		}

		err = session.SaveIndex()
		if err != nil {
			fmt.Printf("Error saving cache index: %v\n", err)
			os.Exit(1)
		}
		exp.Close()
		expFile.Close()

	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
