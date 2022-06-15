package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:     "install [mod]",
	Short:   "Install a mod from a github URL",
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
		if strings.HasSuffix(args[0], "/") {
			fmt.Println("Url cant have a leading slash!")
			os.Exit(1)
		}

		slug := strings.Replace(args[0], "https://github.com/", "", 1)

		if len(strings.Split(args[0], "/")) == 1 {
			slug = args[0]
		}

		if strings.Contains(slug, "/releases") {
			slug = strings.Split(slug, "/releases")[0]
		}

		mod, err := fetchMod(slug)

		installMod(mod, pack)
	},
}

func init() {
	githubCmd.AddCommand(installCmd)
}

const githubApiUrl = "https://api.github.com/"

func fetchMod(slug string) (Mod, error) {
	var modReleases []ModReleases
	var mod Mod
	resp, err := http.Get(githubApiUrl + "repos/" + slug + "/releases")
	if err != nil {
		return mod, err
	}

	if resp.StatusCode == 404 {
		return mod, fmt.Errorf("mod not found (for URL %v)", githubApiUrl+"repos/"+slug+"/releases")
	}

	if resp.StatusCode != 200 {
		return mod, fmt.Errorf("invalid response status %v for URL %v", resp.Status, githubApiUrl+"repos/"+slug+"/releases")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return mod, err
	}

	err = json.Unmarshal(body, &modReleases)
	if err != nil {
		return mod, err
	}

	var repoData Repo

	repoResp, err := http.Get(githubApiUrl + "repos/" + slug)

	if err != nil {
		return mod, err
	}

	defer repoResp.Body.Close()

	repoBody, err := ioutil.ReadAll(repoResp.Body)
	if err != nil {
		return mod, err
	}

	err = json.Unmarshal(repoBody, &repoData)
	if err != nil {
		return mod, err
	}

	release := modReleases[0]

	mod = Mod{
		ID:          repoData.Name,
		Slug:        slug,
		Team:        repoData.Owner.Login,
		Title:       repoData.Name,
		Description: repoData.Description,
		Published:   repoData.CreatedAt,
		Updated:     release.CreatedAt,
		License:     repoData.License,
		ClientSide:  "unknown",
		ServerSide:  "unknown",
		Categories:  repoData.Topics,
	}
	if mod.ID == "" {
		return mod, errors.New("invalid json whilst fetching mod: " + slug)
	}

	return mod, nil

}

func installMod(mod Mod, pack core.Pack) error {
	fmt.Printf("Found mod %s: '%s'.\n", mod.Title, mod.Description)

	latestVersion, err := getLatestVersion(mod.Slug, pack, "")
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}
	if latestVersion.URL == "" {
		return errors.New("mod is not available for this Minecraft version (use the acceptable-game-versions option to accept more) or mod loader")
	}

	return installVersion(mod, latestVersion, pack)
}

func getLatestVersion(slug string, pack core.Pack, branch string) (ModReleases, error) {
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
		ModID:            mod.Slug,
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

type Asset struct {
	URL      string      `json:"url"`
	ID       int         `json:"id"`
	NodeID   string      `json:"node_id"`
	Name     string      `json:"name"`
	Label    interface{} `json:"label"`
	Uploader struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"uploader"`
	ContentType        string    `json:"content_type"`
	State              string    `json:"state"`
	Size               int       `json:"size"`
	DownloadCount      int       `json:"download_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	BrowserDownloadURL string    `json:"browser_download_url"`
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

type Repo struct {
	ID       int    `json:"id"`
	NodeID   string `json:"node_id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	Owner    struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"owner"`
	HTMLURL          string      `json:"html_url"`
	Description      string      `json:"description"`
	Fork             bool        `json:"fork"`
	URL              string      `json:"url"`
	ForksURL         string      `json:"forks_url"`
	KeysURL          string      `json:"keys_url"`
	CollaboratorsURL string      `json:"collaborators_url"`
	TeamsURL         string      `json:"teams_url"`
	HooksURL         string      `json:"hooks_url"`
	IssueEventsURL   string      `json:"issue_events_url"`
	EventsURL        string      `json:"events_url"`
	AssigneesURL     string      `json:"assignees_url"`
	BranchesURL      string      `json:"branches_url"`
	TagsURL          string      `json:"tags_url"`
	BlobsURL         string      `json:"blobs_url"`
	GitTagsURL       string      `json:"git_tags_url"`
	GitRefsURL       string      `json:"git_refs_url"`
	TreesURL         string      `json:"trees_url"`
	StatusesURL      string      `json:"statuses_url"`
	LanguagesURL     string      `json:"languages_url"`
	StargazersURL    string      `json:"stargazers_url"`
	ContributorsURL  string      `json:"contributors_url"`
	SubscribersURL   string      `json:"subscribers_url"`
	SubscriptionURL  string      `json:"subscription_url"`
	CommitsURL       string      `json:"commits_url"`
	GitCommitsURL    string      `json:"git_commits_url"`
	CommentsURL      string      `json:"comments_url"`
	IssueCommentURL  string      `json:"issue_comment_url"`
	ContentsURL      string      `json:"contents_url"`
	CompareURL       string      `json:"compare_url"`
	MergesURL        string      `json:"merges_url"`
	ArchiveURL       string      `json:"archive_url"`
	DownloadsURL     string      `json:"downloads_url"`
	IssuesURL        string      `json:"issues_url"`
	PullsURL         string      `json:"pulls_url"`
	MilestonesURL    string      `json:"milestones_url"`
	NotificationsURL string      `json:"notifications_url"`
	LabelsURL        string      `json:"labels_url"`
	ReleasesURL      string      `json:"releases_url"`
	DeploymentsURL   string      `json:"deployments_url"`
	CreatedAt        string      `json:"created_at"`
	UpdatedAt        string      `json:"updated_at"`
	PushedAt         string      `json:"pushed_at"`
	GitURL           string      `json:"git_url"`
	SSHURL           string      `json:"ssh_url"`
	CloneURL         string      `json:"clone_url"`
	SvnURL           string      `json:"svn_url"`
	Homepage         string      `json:"homepage"`
	Size             int         `json:"size"`
	StargazersCount  int         `json:"stargazers_count"`
	WatchersCount    int         `json:"watchers_count"`
	Language         string      `json:"language"`
	HasIssues        bool        `json:"has_issues"`
	HasProjects      bool        `json:"has_projects"`
	HasDownloads     bool        `json:"has_downloads"`
	HasWiki          bool        `json:"has_wiki"`
	HasPages         bool        `json:"has_pages"`
	ForksCount       int         `json:"forks_count"`
	MirrorURL        interface{} `json:"mirror_url"`
	Archived         bool        `json:"archived"`
	Disabled         bool        `json:"disabled"`
	OpenIssuesCount  int         `json:"open_issues_count"`
	License          struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		SpdxID string `json:"spdx_id"`
		URL    string `json:"url"`
		NodeID string `json:"node_id"`
	} `json:"license"`
	AllowForking   bool        `json:"allow_forking"`
	IsTemplate     bool        `json:"is_template"`
	Topics         []string    `json:"topics"`
	Visibility     string      `json:"visibility"`
	Forks          int         `json:"forks"`
	OpenIssues     int         `json:"open_issues"`
	Watchers       int         `json:"watchers"`
	DefaultBranch  string      `json:"default_branch"`
	TempCloneToken interface{} `json:"temp_clone_token"`
	Organization   struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"organization"`
	NetworkCount     int `json:"network_count"`
	SubscribersCount int `json:"subscribers_count"`
}
