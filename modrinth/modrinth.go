package modrinth

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/comp500/packwiz/cmd"
	"github.com/comp500/packwiz/core"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var modrinthApiUrl = "https://api.modrinth.com/api/v1/"

var modrinthCmd = &cobra.Command{
	Use:     "modrinth",
	Aliases: []string{"mr"},
	Short:   "Manage modrinth-based mods",
}

func init() {
	cmd.Add(modrinthCmd)
	core.Updaters["modrinth"] = mrUpdater{}
}

type License struct {
	Id   string `json:"id"`   //The license id of a mod, retrieved from the licenses get route
	Name string `json:"name"` //The long for name of a license
	Url  string `json:"url"`  //The URL to this license
}

type Mod struct {
	ID          string   `json:"id"`          //The ID of the mod, encoded as a base62 string
	Slug        string   `json:"slug"`        //The slug of a mod, used for vanity URLs
	Team        string   `json:"team"`        //The id of the team that has ownership of this mod
	Title       string   `json:"title"`       //The title or name of the mod
	Description string   `json:"description"` //A short description of the mod
	Body        string   `json:"body"`        //A long form description of the mod.
	body_url    string   `json:"body_url"`    //DEPRECATED The link to the long description of the mod (Optional)
	Published   string   `json:"published"`   //The date at which the mod was first published
	Updated     string   `json:"updated"`     //The date at which the mod was updated
	Status      string   `json:"status"`      //The status of the mod - approved, rejected, draft, unlisted, processing, or unknown
	License     string   `json:"license"`     //The license of the mod
	ClientSide  string   `json:"client_side"` //The support range for the client mod - required, optional, unsupported, or unknown
	ServerSide  string   `json:"server_side"` //The support range for the server mod - required, optional, unsupported, or unknown
	Downloads   string   `json:"downloads"`   //The total number of downloads the mod has
	Categories  []string `json:"categories"`  //A list of the categories that the mod is in
	Versions    []string `json:"versions"`    //A list of ids for versions of the mod
	IconUrl     string   `json:"icon_url"`    //The URL of the icon of the mod (Optional)
	IssuesUrl   string   `json:"issues_url"`  //An optional link to where to submit bugs or issues with the mod (Optional)
	SourceUrl   string   `json:"source_url"`  //An optional link to the source code for the mod (Optional)
	WikiUrl     string   `json:"wiki_url"`    //An optional link to the mod's wiki page or other relevant information (Optional)
	DiscordUrl  string   `json:"discord_url"` //An optional link to the mod's discord (Optional)
}

type ModResult struct {
	ModID         string   `json:"mod_id"`         //The id of the mod; prefixed with local-
	ProjectType   string   `json:"project_id"`     //The project type of the mod
	Author        string   `json:"author"`         //The username of the author of the mod
	Title         string   `json:"title"`          //The name of the mod
	Description   string   `json:"description"`    //A short description of the mod
	Categories    []string `json:"categories"`     //A list of the categories the mod is in
	Versions      []string `json:"versions"`       //A list of the minecraft versions supported by the mod
	Downloads     int      `json:"downloads"`      //The total number of downloads for the mod
	PageUrl       string   `json:"page_url"`       //A link to the mod's main page;
	IconUrl       string   `json:"icon_url"`       //The url of the mod's icon
	AuthorUrl     string   `json:"author_url"`     //The url of the mod's author
	DateCreated   string   `json:"date_created"`   //The date that the mod was originally created
	DateModified  string   `json:"date_modified"`  //The date that the mod was last modified
	LatestVersion string   `json:"latest_version"` //The latest version of minecraft that this mod supports
	License       string   `json:"license"`        //The id of the license this mod follows
	ClientSide    string   `json:"client_side"`    //The side type id that this mod is on the client
	ServerSide    string   `json:"server_side"`    //The side type id that this mod is on the server
	Host          string   `json:"host"`           //The host that this mod is from, always modrinth
}

type ModSearchResult struct {
	Hits      []ModResult `json:"hits"`       //The list of results
	Offset    int         `json:"offset"`     //The number of results that were skipped by the query
	Limit     int         `json:"limit"`      //The number of mods returned by the query
	TotalHits int         `json:"total_hits"` //The total number of mods that the query found
}

type Version struct {
	ID            string        `json:"id"`             //The ID of the version, encoded as a base62 string
	ModID         string        `json:"mod_id"`         //The ID of the mod this version is for
	AuthorId      string        `json:"author_id"`      //The ID of the author who published this version
	Featured      bool          `json:"featured"`       //Whether the version is featured or not
	Name          string        `json:"name"`           //The name of this version
	VersionNumber string        `json:"version_number"` //The version number. Ideally will follow semantic versioning
	Changelog     string        `json:"changelog"`      //The changelog for this version of the mod. (Optional)
	DatePublished string        `json:"date_published"` //The date that this version was published
	Downloads     int           `json:"downloads"`      //The number of downloads this specific version has
	VersionType   []string      `json:"version_type"`   //The type of the release - alpha, beta, or release
	Files         []VersionFile `json:"files"`          //A list of files available for download for this version
	Dependencies  []string      `json:"dependencies"`   //A list of specific versions of mods that this version depends on
	GameVersions  []string      `json:"game_versions"`  //A list of versions of Minecraft that this version of the mod supports
	Loaders       []string      `json:"loaders"`        //The mod loaders that this version supports
}

type VersionFile struct {
	Hashes   map[string]string //A map of hashes of the file. The key is the hashing algorithm and the value is the string version of the hash.
	Url      string            //A direct link to the file
	Filename string            //The name of the file
}

func getFirstModIdViaSearch(query string, version string) (ModResult, error) {
	var null ModResult

	baseUrl, err := url.Parse(modrinthApiUrl)
	baseUrl.Path += "mod"

	params := url.Values{}
	params.Add("limit", "1")
	params.Add("index", "relevance")
	params.Add("facets", "[[\"versions:"+version+"\"]]")
	params.Add("query", query)

	baseUrl.RawQuery = params.Encode()

	resp, err := http.Get(baseUrl.String())
	if err != nil {
		return null, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return null, err
	}

	var result ModSearchResult
	json.Unmarshal(body, &result)

	if result.TotalHits <= 0 {
		return null, errors.New("Couldn't find that mod for this version.")
	}

	return result.Hits[0], nil
}

func fetchMod(modId string) (Mod, error) {
	var mod Mod

	baseUrl, err := url.Parse(modrinthApiUrl)
	baseUrl.Path += "mod/"
	baseUrl.Path += modId

	resp, err := http.Get(baseUrl.String())
	if err != nil {
		return mod, err
	}

	if resp.StatusCode == 404 {
		return mod, errors.New("Couldn't find version: " + modId)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return mod, err
	}

	json.Unmarshal(body, &mod)

	if mod.ID == "" {
		return mod, errors.New("Invalid json whilst fetching mod: " + modId)
	}

	return mod, nil
}

func fetchVersion(versionId string) (Version, error) {
	var version Version
	baseUrl, err := url.Parse(modrinthApiUrl)
	baseUrl.Path += "version/"
	baseUrl.Path += versionId

	resp, err := http.Get(baseUrl.String())
	if err != nil {
		return version, err
	}

	if resp.StatusCode == 404 {
		return version, errors.New("Couldn't find version: " + versionId)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return version, err
	}

	json.Unmarshal(body, &version)

	if version.ID == "" {
		return version, errors.New("Invalid json whilst fetching version: " + versionId)
	}

	return version, nil
}

func (mod Mod) fetchAllVersions() ([]Version, error) {
	ret := make([]Version, len(mod.Versions))

	for i, v := range mod.Versions {
		version, err := fetchVersion(v)
		if err != nil {
			return ret, err
		}

		ret[i] = version
	}
	return ret, nil
}

func (v Version) isValid(mcVersion string, dependencies map[string]string) bool {
	return v.containsVersion(mcVersion) && v.containsAnyLoader(dependencies)
}

func (v Version) containsVersion(mcVersion string) bool {
	for _, v := range v.GameVersions {
		if strings.EqualFold(v, mcVersion) {
			return true
		}
	}
	return false
}

func (v Version) containsAnyLoader(dependencies map[string]string) bool {
	for dependency := range dependencies {
		if v.containsLoader(dependency) {
			return true
		}
	}
	return false
}

func (v Version) containsLoader(modLoader string) bool {
	for _, v := range v.Loaders {
		if strings.EqualFold(v, modLoader) {
			return true
		}
	}
	return false
}

func (mod Mod) getSide() string {
	server := shouldDownloadOnSide(mod.ServerSide)
	client := shouldDownloadOnSide(mod.ClientSide)

	if server && client {
		return core.UniversalSide
	} else if server {
		return core.ServerSide
	} else if client {
		return core.ClientSide
	} else {
		return ""
	}
}

func shouldDownloadOnSide(side string) bool {
	return side == "required" || side == "optional"
}

func (mod Mod) fetchAndGetLatestVersion(pack core.Pack) (Version, error) {
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return Version{}, err
	}

	dependencies := pack.Versions

	versions, err := mod.fetchAllVersions()
	if err != nil {
		return Version{}, err
	}

	var latestValidVersion Version
	for _, v := range versions {
		if v.isValid(mcVersion, dependencies) {
			var semverCompare = semver.Compare(v.VersionNumber, latestValidVersion.VersionNumber)
			if semverCompare == 0 {
				//Semver is equal, compare date instead
				vDate, _ := time.Parse(time.RFC3339Nano, v.DatePublished)
				latestDate, _ := time.Parse(time.RFC3339Nano, latestValidVersion.DatePublished)
				if vDate.After(latestDate) {
					latestValidVersion = v
				}
			} else if semverCompare == 1 {
				latestValidVersion = v
			}
		}
	}

	if latestValidVersion.ID == "" {
		return Version{}, errors.New("Mod is not available for this minecraft version or mod loader.")
	}

	return latestValidVersion, nil
}

func (v VersionFile) getBestHash() (string, string) {
	//try preferred hashes first
	val, exists := v.Hashes["sha256"]
	if exists {
		return "sha256", val
	}
	val, exists = v.Hashes["murmur2"]
	if exists {
		return "murmur2", val
	}
	val, exists = v.Hashes["sha512"]
	if exists {
		return "sha512", val
	}

	//none of the preferred hashes are present, just get the first one
	for key, val := range v.Hashes {
		return key, val
	}

	//No hashes were present
	return "", ""
}
