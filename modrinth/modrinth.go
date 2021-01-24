package modrinth

import (
    "encoding/json"
    "io/ioutil"
    "net/http"
    "net/url"
    "errors"
	"time"
    "strings"

	"golang.org/x/mod/semver"
    "github.com/spf13/cobra"
	"github.com/comp500/packwiz/cmd"
    "github.com/comp500/packwiz/core"
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
    Id string   //The license id of a mod, retrieved from the licenses get route
    Name string //The long for name of a license
    Url string  //The URL to this license
}

type Mod struct {
    Id string           //The ID of the mod, encoded as a base62 string
    Slug string         //The slug of a mod, used for vanity URLs
    Team string         //The id of the team that has ownership of this mod
    Title string        //The title or name of the mod
    Description string  //A short description of the mod
    Body string         //A long form description of the mod.
    body_url string     //DEPRECATED The link to the long description of the mod (Optional)
    Published string    //The date at which the mod was first published
    Updated string      //The date at which the mod was updated
    Status string       //The status of the mod - approved, rejected, draft, unlisted, processing, or unknown
    License string      //The license of the mod
    Client_side string  //The support range for the client mod - required, optional, unsupported, or unknown
    Server_side string  //The support range for the server mod - required, optional, unsupported, or unknown
    Downloads string    //The total number of downloads the mod has
    Categories []string //A list of the categories that the mod is in
    Versions []string   //A list of ids for versions of the mod
    Icon_url string     //The URL of the icon of the mod (Optional)
    Issues_url string   //An optional link to where to submit bugs or issues with the mod (Optional)
    Source_url string   //An optional link to the source code for the mod (Optional)
    Wiki_url string     //An optional link to the mod's wiki page or other relevant information (Optional)
    Discord_url string  //An optional link to the mod's discord (Optional)
    //Donation_urls []DonationPlatform //An optional list of all donation links the mod has (Optional)
}

type ModResult struct {
    Mod_id string 	      //The id of the mod; prefixed with local-
    Project_type string   //The project type of the mod
    Author string 	      //The username of the author of the mod
    Title string 	      //The name of the mod
    Description string    //A short description of the mod
    Categories []string   //A list of the categories the mod is in
    Versions []string     //A list of the minecraft versions supported by the mod
    Downloads int         //The total number of downloads for the mod
    Page_url string       //A link to the mod's main page;
    Icon_url string       //The url of the mod's icon
    Author_url string     //The url of the mod's author
    Date_created string   //The date that the mod was originally created
    Date_modified string  //The date that the mod was last modified
    Latest_version string //The latest version of minecraft that this mod supports
    License string        //The id of the license this mod follows
    Client_side string    //The side type id that this mod is on the client
    Server_side string    //The side type id that this mod is on the server
    Host string           //The host that this mod is from, always modrinth
}

type ModSearchResult struct {
    Hits []ModResult      //The list of results
    Offset int            //The number of results that were skipped by the query
    Limit int             //The number of mods returned by the query
    Total_hits int        //The total number of mods that the query found
}

type Version struct {
    Id string               //The ID of the version, encoded as a base62 string
    Mod_id string           //The ID of the mod this version is for
    Author_id string        //The ID of the author who published this version
    Featured bool           //Whether the version is featured or not
    Name string             //The name of this version
    Version_number string   //The version number. Ideally will follow semantic versioning
    Changelog string        //The changelog for this version of the mod. (Optional)
    Changelog_url string    //DEPRECATED A link to the changelog for this version of the mod (Optional)
    Date_published string   //The date that this version was published
    Downloads int           //The number of downloads this specific version has
    Version_type []string   //The type of the release - alpha, beta, or release
    Files []VersionFile     //A list of files available for download for this version
    Dependencies []string   //A list of specific versions of mods that this version depends on
    Game_versions []string    //A list of versions of Minecraft that this version of the mod supports
    Loaders []string          //The mod loaders that this version supports
}

type VersionFile struct {
    Hashes map[string]string //A map of hashes of the file. The key is the hashing algorithm and the value is the string version of the hash.
    Url string               //A direct link to the file
    Filename string          //The name of the file
}

func getFirstModIdViaSearch(query string, version string) (ModResult,error) {
    var null ModResult;

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

    var result ModSearchResult;
    json.Unmarshal(body, &result)

    if result.Total_hits <= 0 {
        return null, errors.New("Couldn't find that mod for this version.")
    }

    return result.Hits[0], nil
}

func fetchMod(modId string) (Mod, error) {
    var mod Mod;

    baseUrl, err := url.Parse(modrinthApiUrl)
    baseUrl.Path += "mod/"
    baseUrl.Path += modId

    resp, err := http.Get(baseUrl.String())
    if err != nil {
        return mod, err
    }

    if resp.StatusCode == 404 {
        return mod, errors.New("Couldn't find version: "+modId)
    }

    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return mod, err
    }

    json.Unmarshal(body, &mod)

    if (mod.Id == "") {
        return mod, errors.New("Invalid json whilst fetching mod: "+modId)
    }

    return mod, nil
}

func fetchVersion(versionId string) (Version, error) {
    var version Version;
    baseUrl, err := url.Parse(modrinthApiUrl)
    baseUrl.Path += "version/"
    baseUrl.Path += versionId

    resp, err := http.Get(baseUrl.String())
    if err != nil {
        return version, err
    }

    if resp.StatusCode == 404 {
        return version, errors.New("Couldn't find version: "+versionId)
    }

    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return version, err
    }

    json.Unmarshal(body, &version)

    if (version.Id == "") {
        return version, errors.New("Invalid json whilst fetching version: "+versionId)
    }

    return version, nil
}

func (mod Mod) fetchAllVersions() ([]Version, error) {
    ret := make([]Version, len(mod.Versions))

    for i,v := range mod.Versions {
        version, err := fetchVersion(v)
        if err != nil {
            return ret, err
        }

        ret[i] = version
    }
    return ret, nil
}

func (v Version) isValid(mcVersion string) bool {
    // TODO this should also check if the version is for the correct modLoader
    return v.containsVersion(mcVersion)
}

func (v Version) containsVersion(mcVersion string) bool {
    for _,v := range v.Game_versions {
        if strings.EqualFold(v, mcVersion) {
            return true
        }
    }
    return false
}

func (v Version) containsLoader(modLoader string) bool {
    for _,v := range v.Loaders {
        if strings.EqualFold(v, modLoader) {
            return true
        }
    }
    return false
}

func (mod Mod) getSide() string {
    server := shouldDownloadOnSide(mod.Server_side)
    client := shouldDownloadOnSide(mod.Client_side)

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

func (mod Mod) fetchAndGetLatestVersion(mcVersion string) (Version, error) {
    versions, err := mod.fetchAllVersions()
    if err != nil {
        return Version{}, err
    }

    var latestValidVersion Version;
    for _,v := range versions {
        if v.isValid(mcVersion) {
            var semverCompare = semver.Compare(v.Version_number, latestValidVersion.Version_number)
            if semverCompare == 0 {
                //Semver is equal, compare date instead
                vDate, _ := time.Parse(time.RFC3339Nano, v.Date_published)
                latestDate, _ := time.Parse(time.RFC3339Nano, latestValidVersion.Date_published)
                if (vDate.After(latestDate)) {
                    latestValidVersion = v
                }
            } else if semverCompare == 1 {
                latestValidVersion = v
            }
        }
    }

    if latestValidVersion.Id == "" {
        return Version{},errors.New("Mod is not available for this minecraft version.")
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