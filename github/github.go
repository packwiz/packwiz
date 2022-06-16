package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/packwiz/packwiz/cmd"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
)

var githubCmd = &cobra.Command{
	Use:     "github",
	Aliases: []string{"gh"},
	Short:   "Manage github-based mods",
}

func init() {
	cmd.Add(githubCmd)
	core.Updaters["github"] = ghUpdater{}
}

func fetchRepo(slug string) (Repo, error) {
	var repo Repo
	res, err := http.Get(githubApiUrl + "repos/" + slug)

	if err != nil {
		return repo, err
	}

	defer res.Body.Close()

	repoBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return repo, err
	}

	err = json.Unmarshal(repoBody, &repo)
	if err != nil {
		return repo, err
	}
	return repo, nil
}

func fetchMod(slug string) (Mod, error) {

	var mod Mod

	repo, err := fetchRepo(slug)

	if err != nil {
		return mod, err
	}

	release, err := getLatestVersion(slug, "")

	if err != nil {
		return mod, err
	}

	mod = Mod{
		ID:          repo.Name,
		Slug:        slug,
		Team:        repo.Owner.Login,
		Title:       repo.Name,
		Description: repo.Description,
		Published:   repo.CreatedAt,
		Updated:     release.CreatedAt,
		License:     repo.License,
		ClientSide:  "unknown",
		ServerSide:  "unknown",
		Categories:  repo.Topics,
	}
	if mod.ID == "" {
		return mod, errors.New("invalid json whilst fetching mod: " + slug)
	}

	return mod, nil

}

type ModReleases struct {
	URL             string  `json:"url"`
	NodeID          string  `json:"node_id"`
	TagName         string  `json:"tag_name"`
	TargetCommitish string  `json:"target_commitish"` // The branch of the release
	Name            string  `json:"name"`
	CreatedAt       string  `json:"created_at"`
	Assets          []Asset `json:"assets"`
}

type Asset struct {
	URL                string    `json:"url"`
	Name               string    `json:"name"`
	UpdatedAt          time.Time `json:"updated_at"`
	BrowserDownloadURL string    `json:"browser_download_url"`
}

type Repo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	License     struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		SpdxID string `json:"spdx_id"`
		URL    string `json:"url"`
		NodeID string `json:"node_id"`
	} `json:"license"`
	Topics []string `json:"topics"`
}

func (u ghUpdateData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}

type License struct {
	Id   string `json:"id"`   //The license id of a mod, retrieved from the licenses get route
	Name string `json:"name"` //The long for name of a license
	Url  string `json:"url"`  //The URL to this license
}

type Mod struct {
	ID          string `json:"id"`          //The ID of the mod, encoded as a base62 string
	Slug        string `json:"slug"`        //The slug of a mod, used for vanity URLs
	Team        string `json:"team"`        //The id of the team that has ownership of this mod
	Title       string `json:"title"`       //The title or name of the mod
	Description string `json:"description"` //A short description of the mod
	// Body        string   `json:"body"`        //A long form description of the mod.
	// BodyUrl     string   `json:"body_url"`    //DEPRECATED The link to the long description of the mod (Optional)
	Published string `json:"published"` //The date at which the mod was first published
	Updated   string `json:"updated"`   //The date at which the mod was updated
	License   struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		SpdxID string `json:"spdx_id"`
		URL    string `json:"url"`
		NodeID string `json:"node_id"`
	} `json:"license"`
	ClientSide string `json:"client_side"` //The support range for the client mod - required, optional, unsupported, or unknown
	ServerSide string `json:"server_side"` //The support range for the server mod - required, optional, unsupported, or unknown
	// Downloads  int      `json:"downloads"`   //The total number of downloads the mod has
	Categories []string `json:"categories"`  //A list of the categories that the mod is in
	Versions   []string `json:"versions"`    //A list of ids for versions of the mod
	IconUrl    string   `json:"icon_url"`    //The URL of the icon of the mod (Optional)
	IssuesUrl  string   `json:"issues_url"`  //An optional link to where to submit bugs or issues with the mod (Optional)
	SourceUrl  string   `json:"source_url"`  //An optional link to the source code for the mod (Optional)
	WikiUrl    string   `json:"wiki_url"`    //An optional link to the mod's wiki page or other relevant information (Optional)
	DiscordUrl string   `json:"discord_url"` //An optional link to the mod's discord (Optional)
}

func (u Asset) getSha1() (string, error) {
	// TODO potentionally cache downloads to speed things up and avoid getting ratelimited by github!
	mainHasher, err := core.GetHashImpl("sha1")
	resp, err := http.Get(u.BrowserDownloadURL)
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
