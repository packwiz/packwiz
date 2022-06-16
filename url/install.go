package url

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var installCmd = &cobra.Command{
	Use:     "install [name] [url]",
	Short:   "Add an external file from a direct download link, for sites that are not directly supported by packwiz",
	Aliases: []string{"add", "get"},
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := core.LoadPack()

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		dl, err := url.Parse(args[1])

		if err != nil {
			fmt.Println("Failed parsing URL:", err)
			os.Exit(1)
		}
		if dl.Scheme != "https" && dl.Scheme != "http" {
			fmt.Println("Unsupported url scheme", dl.Scheme)
			os.Exit(1)
		}

		// TODO: consider using colors for these warnings but those can have issues on windows
		force, err := cmd.Flags().GetBool("force")
		if !force && err == nil {
			var msg string
			if dl.Host == "github.com" {
				msg = "github add " + args[1]
				os.Exit(1)
			}
			if dl.Host == "modrinth.com" {
				msg = "modrinth add " + args[1]
			}
			if dl.Host == "www.curseforge.com" || dl.Host == "curseforge.com" {
				msg = "curseforge add " + args[1]
			}
			if msg != "" {
				fmt.Println("Consider using packwiz", msg, "instead if you know what you are doing use --force to install this mod anyway")
				os.Exit(1)
			}
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
	urlCmd.AddCommand(installCmd)

	installCmd.Flags().Bool("force", false, "Force install a file even if the supplied url is supported by packwiz")
}
