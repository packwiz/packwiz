package github

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/mitchellh/mapstructure"
	"github.com/packwiz/packwiz/cmd"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

var githubCmd = &cobra.Command{
	Use:     "github",
	Aliases: []string{"gh"},
	Short:   "Manage projects released on GitHub",
}

func init() {
	cmd.Add(githubCmd)
	core.Updaters["github"] = ghUpdater{}
}

func fetchRepo(slug string) (Repo, error) {
	var repo Repo

	res, err := ghDefaultClient.getRepo(slug)
	if err != nil {
		return repo, err
	}

	defer res.Body.Close()

	repoBody, err := io.ReadAll(res.Body)
	if err != nil {
		return repo, err
	}

	err = json.Unmarshal(repoBody, &repo)
	if err != nil {
		return repo, err
	}

	if repo.FullName == "" {
		return repo, errors.New("invalid json while fetching project: " + slug)
	}

	return repo, nil
}

type Repo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`      // "hello_world"
	FullName string `json:"full_name"` // "owner/hello_world"
}

type Release struct {
	URL             string  `json:"url"`
	TagName         string  `json:"tag_name"`
	TargetCommitish string  `json:"target_commitish"` // The branch of the release
	Name            string  `json:"name"`
	CreatedAt       string  `json:"created_at"`
	Assets          []Asset `json:"assets"`
}

type Asset struct {
	URL                string `json:"url"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Name               string `json:"name"`
}

func (u ghUpdateData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}

func (u Asset) getSha256() (string, error) {
	// TODO potentionally cache downloads to speed things up and avoid getting ratelimited by github!
	mainHasher, err := core.GetHashImpl("sha256")
	if err != nil {
		return "", err
	}

	resp, err := ghDefaultClient.makeGet(u.BrowserDownloadURL)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	mainHasher.Write(body)

	hash := mainHasher.Sum(nil)

	return mainHasher.HashToString(hash), nil
}
