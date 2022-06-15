package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var installCmd = &cobra.Command{
	Use:     "install [name] [url]",
	Short:   "Install a mod from a url this is different from modrinth/curseforge install",
	Aliases: []string{"add", "get"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := core.LoadPack()

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(args) != 2 {
			fmt.Println("Invalid usage: packwiz install [name] [url]")
			os.Exit(1)
		}
		// TODO: consider using colors for these warnings but those can have issues on windows
		if strings.HasPrefix(args[1], "https://github.com/") {
			fmt.Println("Consider using packwiz github add", args[1], "instead")
		}
		if strings.HasPrefix(args[1], "https://modrinth.com/") {
			fmt.Println("Consider using packwiz modrinth add", args[1], "instead")
		}
		if strings.HasPrefix(args[1], "https://www.curseforge.com/") {
			fmt.Println("Consider using packwiz curseforge add", args[1], "instead")
		}
		hash, err := getSha1(args[1])
		if err != nil {
			fmt.Println("Failed to get sha-1 for file. ", err)
			os.Exit(1)
		}

		index, err := pack.LoadIndex()

		filename := strings.Split(args[1], "/")[len(strings.Split(args[1], "/"))-1]
		modMeta := core.Mod{
			Name:     args[0],
			FileName: filename,
			Side:     "unknown",
			Download: core.ModDownload{
				URL:        args[1],
				HashFormat: "sha1",
				Hash:       hash,
			},
		}
		var path string
		folder := viper.GetString("meta-folder")
		if folder == "" {
			folder = "mods"
		}
		path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder, args[0]+core.MetaExtension))

		// If the file already exists, this will overwrite it!!!
		// TODO: Should this be improved?
		// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
		// VCS anyway.

		format, hash, err := modMeta.Write()
		if err != nil {
			return
		}
		err = index.RefreshFileWithHash(path, format, hash, true)
		if err != nil {
			return
		}
		err = index.Write()
		if err != nil {
			return
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			return
		}
		err = pack.Write()
		if err != nil {
			return
		}
		fmt.Println("Successfully installed", args[0], "from url", args[1])

		return

	}}

func getSha1(url string) (string, error) {
	// TODO potentionally cache downloads to speed things up and avoid getting ratelimited by github!
	mainHasher, err := core.GetHashImpl("sha1")
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("Asset not found")
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Invalid response code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	mainHasher.Write(body)

	hash := mainHasher.Sum(nil)

	return mainHasher.HashToString(hash), nil
}

func init() {
	rootCmd.AddCommand(installCmd)
}
