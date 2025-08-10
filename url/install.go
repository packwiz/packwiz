package url

import (
	"fmt"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var installCmd = &cobra.Command{
	Use:     "add [name] [url]",
	Short:   "Add an external file from a direct download link, for sites that are not directly supported by packwiz",
	Aliases: []string{"install", "get"},
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		dl, err := url.Parse(args[1])
		if err != nil {
			fmt.Println("Failed to parse URL:", err)
			os.Exit(1)
		}
		if dl.Scheme != "https" && dl.Scheme != "http" {
			fmt.Println("Unsupported URL scheme:", dl.Scheme)
			os.Exit(1)
		}

		// TODO: consider using colors for these warnings but those can have issues on windows
		force, err := cmd.Flags().GetBool("force")
		if !force && err == nil {
			var msg string
			// TODO: update when github command is added
			// TODO: make this generic?
			//if dl.Host == "www.github.com" || dl.Host == "github.com" {
			//	msg = "github add " + args[1]
			//}
			if strings.HasSuffix(dl.Host, "modrinth.com") {
				msg = "modrinth add " + args[1]
			}
			if strings.HasSuffix(dl.Host, "curseforge.com") || strings.HasSuffix(dl.Host, "forgecdn.net") {
				msg = "curseforge add " + args[1]
			}
			if msg != "" {
				fmt.Println("Consider using packwiz", msg, "instead; if you know what you are doing use --force to add this file without update metadata.")
				os.Exit(1)
			}
		}

		hash, err := getHash(args[1])
		if err != nil {
			fmt.Println("Failed to retrieve SHA256 hash for file", err)
			os.Exit(1)
		}

		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		filename := path.Base(dl.Path)
		modMeta := core.Mod{
			Name:     args[0],
			FileName: filename,
			Side:     core.UniversalSide,
			Download: core.ModDownload{
				URL:        args[1],
				HashFormat: "sha256",
				Hash:       hash,
			},
		}

		folder := viper.GetString("meta-folder")
		if folder == "" {
			folder = "mods"
		}
		destPathName, err := cmd.Flags().GetString("meta-name")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if destPathName == "" {
			destPathName = core.SlugifyName(args[0])
		}
		destPath := modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder,
			destPathName+core.MetaExtension))

		format, hash, err := modMeta.Write()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = index.RefreshFileWithHash(destPath, format, hash, true)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = index.Write()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = pack.UpdateIndexHash()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = pack.Write()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("Successfully added %s (%s) from: %s\n", args[0], destPath, args[1])
	}}

func getHash(url string) (string, error) {
	mainHasher, err := core.GetHashImpl("sha256")
	if err != nil {
		return "", err
	}
	resp, err := core.GetWithUA(url, "application/octet-stream")
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download: unexpected response status: %v", resp.Status)
	}

	_, err = io.Copy(mainHasher, resp.Body)
	if err != nil {
		return "", err
	}

	return mainHasher.HashToString(mainHasher.Sum(nil)), nil
}

func init() {
	urlCmd.AddCommand(installCmd)

	installCmd.Flags().Bool("force", false, "Add a file even if the download URL is supported by packwiz in an alternative command (which may support dependencies and updates)")
	installCmd.Flags().String("meta-name", "", "Filename to use for the created metadata file (defaults to a name generated from the name you supply)")
}
