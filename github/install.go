package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/dlclark/regexp2"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var GithubRegex = regexp.MustCompile(`^https?://(?:www\.)?github\.com/([^/]+/[^/]+)`)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "add [URL|slug]",
	Short:   "Add a project from a GitHub repository URL or slug",
	Aliases: []string{"install", "get"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(args) == 0 || len(args[0]) == 0 {
			fmt.Println("You must specify a GitHub repository URL.")
			os.Exit(1)
		}

		// Try interpreting the argument as a slug, or GitHub repository URL.
		var slug string
		var branch string

		// Regex to match potential release assets against.
		// The default will match any asset with a name that does *not* end with:
		// - "-api.jar"
		// - "-dev.jar"
		// - "-dev-preshadow.jar"
		// - "-sources.jar"
		// In most cases, this will only match one asset.
		// TODO: Hopefully.
		regex := `^.+(?<!-api|-dev|-dev-preshadow|-sources)\.jar$`

		// Check if the argument is a valid GitHub repository URL; if so, extract the slug from the URL.
		// Otherwise, interpret the argument as a slug directly.
		matches := GithubRegex.FindStringSubmatch(args[0])
		if len(matches) == 2 {
			slug = matches[1]
		} else {
			slug = args[0]
		}

		repo, err := fetchRepo(slug)

		if err != nil {
			fmt.Printf("Failed to add project: %s\n", err)
			os.Exit(1)
		}

		if branchFlag != "" {
			branch = branchFlag
		}
		if regexFlag != "" {
			regex = regexFlag
		}

		err = installMod(repo, branch, regex, pack)
		if err != nil {
			fmt.Printf("Failed to add project: %s\n", err)
			os.Exit(1)
		}
	},
}

func installMod(repo Repo, branch string, regex string, pack core.Pack) error {
	latestRelease, err := getLatestRelease(repo.FullName, branch)
	if err != nil {
		return fmt.Errorf("failed to get latest release: %v", err)
	}

	return installRelease(repo, latestRelease, regex, pack)
}

func getLatestRelease(slug string, branch string) (Release, error) {
	var releases []Release
	var release Release

	resp, err := ghDefaultClient.getReleases(slug)
	if err != nil {
		return release, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return release, err
	}

	err = json.Unmarshal(body, &releases)
	if err != nil {
		return release, err
	}

	if branch != "" {
		for _, r := range releases {
			if r.TargetCommitish == branch {
				return r, nil
			}
		}
		return release, fmt.Errorf("failed to find release for branch %v", branch)
	}

	return releases[0], nil
}

func installRelease(repo Repo, release Release, regex string, pack core.Pack) error {
	expr := regexp2.MustCompile(regex, 0)

	if len(release.Assets) == 0 {
		return errors.New("release doesn't have any assets attached")
	}

	var files []Asset

	for _, v := range release.Assets {
		bl, _ := expr.MatchString(v.Name)
		if bl {
			files = append(files, v)
		}
	}

	if len(files) == 0 {
		return errors.New("release doesn't have any assets matching regex")
	}

	if len(files) > 1 {
		// TODO: also print file names
		return errors.New("release has more than one asset matching regex")
	}

	file := files[0]

	// Install the file
	fmt.Printf("Installing %s from release %s\n", file.Name, release.TagName)
	index, err := pack.LoadIndex()
	if err != nil {
		return err
	}

	updateMap := make(map[string]map[string]interface{})

	updateMap["github"], err = ghUpdateData{
		Slug:   repo.FullName,
		Tag:    release.TagName,
		Branch: release.TargetCommitish, // TODO: if no branch is specified by the user, we shouldn't record it - in order to remain branch-agnostic in getLatestRelease()
		Regex:  regex,                   // TODO: ditto!
	}.ToMap()
	if err != nil {
		return err
	}

	hash, err := file.getSha256()
	if err != nil {
		return err
	}

	modMeta := core.Mod{
		Name:     repo.Name,
		FileName: file.Name,
		Side:     core.UniversalSide,
		Download: core.ModDownload{
			URL:        file.BrowserDownloadURL,
			HashFormat: "sha256",
			Hash:       hash,
		},
		Update: updateMap,
	}
	var path string
	folder := viper.GetString("meta-folder")
	if folder == "" {
		folder = "mods"
	}
	path = modMeta.SetMetaPath(filepath.Join(viper.GetString("meta-folder-base"), folder, core.SlugifyName(repo.Name)+core.MetaExtension))

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

	fmt.Printf("Project \"%s\" successfully added! (%s)\n", repo.Name, file.Name)
	return nil
}

var branchFlag string
var regexFlag string

func init() {
	githubCmd.AddCommand(installCmd)

	installCmd.Flags().StringVar(&branchFlag, "branch", "", "The GitHub repository branch to retrieve releases for")
	installCmd.Flags().StringVar(&regexFlag, "regex", "", "The regular expression to match releases against")
}
