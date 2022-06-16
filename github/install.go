package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var GithubRegex = regexp.MustCompile("https?://(?:www\\.)?github\\.com/([^/]+/[^/]+)")

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "install [mod]",
	Short:   "Install mods from github releases",
	Aliases: []string{"add", "get"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := core.LoadPack()

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(args) == 0 || len(args[0]) == 0 {
			fmt.Println("You must specify a mod.")
			os.Exit(1)
		}

		//Try interpreting the arg as a modId or slug.
		//Modrinth transparently handles slugs/mod ids in their api; we don't have to detect which one it is.
		var slug string

		//Try to see if it's a site, if extract the id/slug from the url.
		//Otherwise, interpret the arg as a id/slug straight up
		matches := GithubRegex.FindStringSubmatch(args[0])
		if matches != nil && len(matches) == 2 {
			slug = matches[1]
		} else {
			slug = args[0]
		}

		mod, err := fetchMod(slug)

		if err != nil {
			fmt.Println("Failed to get the mod ", err)
			os.Exit(1)
		}

		installMod(mod, pack)
	},
}

func init() {
	githubCmd.AddCommand(installCmd)
}

const githubApiUrl = "https://api.github.com/"

func installMod(mod Mod, pack core.Pack) error {
	fmt.Printf("Found repo %s: '%s'.\n", mod.Slug, mod.Description)

	latestVersion, err := getLatestVersion(mod.Slug, "")
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}
	if latestVersion.URL == "" {
		return errors.New("mod is not available for this Minecraft version (use the acceptable-game-versions option to accept more) or mod loader")
	}

	return installVersion(mod, latestVersion, pack)
}

func getLatestVersion(slug string, branch string) (ModReleases, error) {
	var modReleases []ModReleases
	var release ModReleases

	resp, err := http.Get(githubApiUrl + "repos/" + slug + "/releases")
	if err != nil {
		return release, err
	}

	if resp.StatusCode == 404 {
		return release, fmt.Errorf("mod not found (for URL %v)", githubApiUrl+"repos/"+slug+"/releases")
	}

	if resp.StatusCode != 200 {
		return release, fmt.Errorf("invalid response status %v for URL %v", resp.Status, githubApiUrl+"repos/"+slug+"/releases")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return release, err
	}
	err = json.Unmarshal(body, &modReleases)
	if err != nil {
		return release, err
	}
	for _, r := range modReleases {
		if r.TargetCommitish == branch {
			return r, nil
		}
	}

	return modReleases[0], nil
}

func installVersion(mod Mod, version ModReleases, pack core.Pack) error {
	var files = version.Assets

	if len(files) == 0 {
		return errors.New("version doesn't have any files attached")
	}

	// TODO: add some way to allow users to pick which file to install?
	var file = files[0]
	for _, v := range version.Assets {
		if strings.HasSuffix(v.Name, ".jar") {
			file = v
		}
	}

	//Install the file
	fmt.Printf("Installing %s from version %s\n", file.URL, version.Name)
	index, err := pack.LoadIndex()
	if err != nil {
		return err
	}

	updateMap := make(map[string]map[string]interface{})

	updateMap["github"], err = ghUpdateData{
		Slug:             mod.Slug,
		InstalledVersion: version.TagName,
		Branch:           version.TargetCommitish,
	}.ToMap()
	if err != nil {
		return err
	}

	hash, error := file.getSha1()
	if error != nil || hash == "" {
		return errors.New("file doesn't have a hash")
	}

	modMeta := core.Mod{
		Name:     mod.Title,
		FileName: file.Name,
		Side:     "unknown",
		Download: core.ModDownload{
			URL:        file.BrowserDownloadURL,
			HashFormat: "sha1",
			Hash:       hash,
		},
		Update: updateMap,
	}
	var path string
	folder := viper.GetString("meta-folder")
	if folder == "" {
		folder = "mods"
	}
	path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder, mod.Title+core.MetaExtension))

	// If the file already exists, this will overwrite it!!!
	// TODO: Should this be improved?
	// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
	// VCS anyway.

	format, hash, err := modMeta.Write()
	if err != nil {
		return err
	}
	err = index.RefreshFileWithHash(path, format, hash, true)
	if err != nil {
		return err
	}
	err = index.Write()
	if err != nil {
		return err
	}
	err = pack.UpdateIndexHash()
	if err != nil {
		return err
	}
	err = pack.Write()
	if err != nil {
		return err
	}
	return nil
}
