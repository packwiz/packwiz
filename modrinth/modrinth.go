package modrinth

import (
	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"errors"
	"fmt"
	"github.com/packwiz/packwiz/cmd"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/unascribed/FlexVer/go/flexver"
	"golang.org/x/exp/slices"
	"math"
	"net/http"
	"net/url"
	"regexp"
)

var modrinthCmd = &cobra.Command{
	Use:     "modrinth",
	Aliases: []string{"mr"},
	Short:   "Manage modrinth-based mods",
}

var mrDefaultClient = modrinthApi.NewClient(&http.Client{})

func init() {
	cmd.Add(modrinthCmd)
	core.Updaters["modrinth"] = mrUpdater{}

	mrDefaultClient.UserAgent = core.UserAgent
}

func getProjectIdsViaSearch(query string, versions []string) ([]*modrinthApi.SearchResult, error) {
	facets := make([]string, 0)
	for _, v := range versions {
		facets = append(facets, "versions:"+v)
	}

	res, err := mrDefaultClient.Projects.Search(&modrinthApi.SearchOptions{
		Limit:  5,
		Index:  "relevance",
		Facets: [][]string{facets},
		Query:  query,
	})

	if err != nil {
		return nil, err
	}
	return res.Hits, nil
}

// "Loaders" that are supported regardless of the configured mod loaders
var defaultMRLoaders = []string{
	// TODO: check if Canvas/Iris/Optifine are installed? suggest installing them?
	"canvas",
	"iris",
	"optifine",
	"vanilla",   // Core shaders
	"minecraft", // Resource packs
}

var withDatapackPathMRLoaders = []string{
	"canvas",
	"iris",
	"optifine",
	"vanilla",   // Core shaders
	"minecraft", // Resource packs
	// TODO: check if a datapack loader is installed; suggest installing one?
	"datapack", // Datapacks (requires a datapack loader)
}

var loaderFolders = map[string]string{
	"quilt":      "mods",
	"fabric":     "mods",
	"forge":      "mods",
	"liteloader": "mods",
	"modloader":  "mods",
	"rift":       "mods",
	"bukkit":     "plugins",
	"spigot":     "plugins",
	"paper":      "plugins",
	"purpur":     "plugins",
	"sponge":     "plugins",
	"bungeecord": "plugins",
	"waterfall":  "plugins",
	"velocity":   "plugins",
	"canvas":     "resourcepacks",
	"iris":       "shaderpacks",
	"optifine":   "shaderpacks",
	"vanilla":    "resourcepacks",
}

// Preference list for loader types, for comparing files where the version is the same - more preferred is lower
var loaderPreferenceList = []string{
	// Prefer quilt versions over fabric versions
	"quilt",
	"fabric",
	"forge",
	"liteloader",
	"modloader",
	"rift",
	// Prefer mods to plugins
	"sponge",
	// Prefer newer Bukkit forks
	"purpur",
	"paper",
	"spigot",
	"bukkit",
	"velocity",
	// Prefer newer BungeeCord forks
	"waterfall",
	"bungeecord",
	// Prefer Canvas shaders to Iris shaders to Optifine shaders to core shaders
	"canvas",
	"iris",
	"optifine",
	"vanilla",
	// Prefer mods to datapacks
	"datapack",
	// Prefer mods to resource packs?! Idk this is just here for completeness
	"minecraft",
}

func getMinLoaderIdx(loaders []string) (minIdx int) {
	minIdx = math.MaxInt
	for _, v := range loaders {
		idx := slices.Index(loaderPreferenceList, v)
		if idx != -1 && idx < minIdx {
			minIdx = idx
		}
	}
	return minIdx
}

func getProjectTypeFolder(projectType string, fileLoaders []string, packLoaders []string) (string, error) {
	if projectType == "modpack" {
		return "", errors.New("this command should not be used to add Modrinth modpacks, and importing of Modrinth modpacks is not yet supported")
	} else if projectType == "resourcepack" {
		return "resourcepacks", nil
	} else if projectType == "shader" {
		bestLoaderIdx := math.MaxInt
		for _, v := range fileLoaders {
			idx := slices.Index(loaderPreferenceList, v)
			if idx != -1 && idx < bestLoaderIdx {
				bestLoaderIdx = idx
			}
		}
		if bestLoaderIdx > -1 && bestLoaderIdx < math.MaxInt {
			return loaderFolders[loaderPreferenceList[bestLoaderIdx]], nil
		}
		return "shaderpacks", nil
	} else if projectType == "mod" {
		// Look up pack loaders in the list of loaders (note this is currently filtered to quilt/fabric/forge)
		bestLoaderIdx := math.MaxInt
		for _, v := range fileLoaders {
			if slices.Contains(packLoaders, v) {
				idx := slices.Index(loaderPreferenceList, v)
				if idx != -1 && idx < bestLoaderIdx {
					bestLoaderIdx = idx
				}
			}
		}
		if bestLoaderIdx > -1 && bestLoaderIdx < math.MaxInt {
			return loaderFolders[loaderPreferenceList[bestLoaderIdx]], nil
		}

		// Datapack loader is "datapack"
		if slices.Contains(fileLoaders, "datapack") {
			if viper.GetString("datapack-folder") != "" {
				return viper.GetString("datapack-folder"), nil
			} else {
				return "", errors.New("set the datapack-folder option to use datapacks")
			}
		}
		// Default to "mods" for mod type
		return "mods", nil
	} else {
		return "", fmt.Errorf("unknown project type %s", projectType)
	}
}

var urlRegexes = [...]*regexp.Regexp{
	// Slug/version number regex from https://github.com/modrinth/labrinth/blob/1679a3f844497d756d0cf272c5374a5236eabd42/src/util/validate.rs#L8
	regexp.MustCompile("^https?://modrinth\\.com/(?P<urlCategory>[^/]+)/(?P<slug>[a-zA-Z0-9!@$()`.+,_\"-]{3,64})(?:/version/(?P<version>[a-zA-Z0-9!@$()`.+,_\"-]{1,32}))?"),
	// Version/project IDs are more restrictive: [a-zA-Z0-9]+ (base62)
	regexp.MustCompile("^https?://cdn\\.modrinth\\.com/data/(?P<slug>[a-zA-Z0-9]+)/versions/(?P<versionID>[a-zA-Z0-9]+)/(?P<filename>[^/]+)$"),
	regexp.MustCompile("^(?P<slug>[a-zA-Z0-9!@$()`.+,_\"-]{3,64})$"),
}

const slugRegexIdx = 2

var urlCategories = []string{
	"mod", "plugin", "datapack", "shader", "resourcepack", "modpack",
}

func parseSlugOrUrl(input string, slug *string, version *string, versionID *string, filename *string) (parsedSlug bool, err error) {
	for regexIdx, r := range urlRegexes {
		matches := r.FindStringSubmatch(input)
		if matches != nil {
			if i := r.SubexpIndex("urlCategory"); i >= 0 {
				if !slices.Contains(urlCategories, matches[i]) {
					err = errors.New("unknown project type: " + matches[i])
					return
				}
			}
			if i := r.SubexpIndex("slug"); i >= 0 {
				*slug = matches[i]
			}
			if i := r.SubexpIndex("version"); i >= 0 {
				*version = matches[i]
			}
			if i := r.SubexpIndex("versionID"); i >= 0 {
				*versionID = matches[i]
			}
			if i := r.SubexpIndex("filename"); i >= 0 {
				var parsed string
				parsed, err = url.PathUnescape(matches[i])
				if err != nil {
					return
				}
				*filename = parsed
			}
			parsedSlug = regexIdx == slugRegexIdx
			return
		}
	}
	return
}

func findLatestVersion(versions []*modrinthApi.Version, gameVersions []string, useFlexVer bool) *modrinthApi.Version {
	latestValidVersion := versions[0]
	latestValidLoaderIdx := getMinLoaderIdx(versions[0].Loaders)
	bestGameVersion := core.HighestSliceIndex(gameVersions, versions[0].GameVersions)
	for _, v := range versions[1:] {
		loaderIdx := getMinLoaderIdx(v.Loaders)
		gameVersionIdx := core.HighestSliceIndex(gameVersions, v.GameVersions)

		var compare int32
		if useFlexVer {
			// Use FlexVer to compare versions
			compare = flexver.Compare(*v.VersionNumber, *latestValidVersion.VersionNumber)
		}

		if compare == 0 {
			// Prefer later specified game versions (main version specified last)
			compare = int32(gameVersionIdx - bestGameVersion)
		}
		if compare == 0 {
			// Prefer loaders; principally Quilt over Fabric, mods over datapacks (Modrinth backend handles filtering)
			compare = int32(latestValidLoaderIdx - loaderIdx)
		}
		if compare == 0 {
			// Other comparisons are equal, compare date instead
			if v.DatePublished.After(*latestValidVersion.DatePublished) {
				compare = 1
			}
		}
		if compare > 0 {
			latestValidVersion = v
			latestValidLoaderIdx = loaderIdx
			bestGameVersion = gameVersionIdx
		}
	}

	return latestValidVersion
}

func getLatestVersion(projectID string, name string, pack core.Pack) (*modrinthApi.Version, error) {
	gameVersions, err := pack.GetSupportedMCVersions()
	if err != nil {
		return nil, err
	}
	var loaders []string
	if viper.GetString("datapack-folder") != "" {
		loaders = append(pack.GetLoaders(), withDatapackPathMRLoaders...)
	} else {
		loaders = append(pack.GetLoaders(), defaultMRLoaders...)
	}

	result, err := mrDefaultClient.Versions.ListVersions(projectID, modrinthApi.ListVersionsOptions{
		GameVersions: gameVersions,
		Loaders:      loaders,
	})

	if len(result) == 0 {
		// TODO: retry with datapack specified, to determine what the issue is? or just request all and filter afterwards
		return nil, errors.New("no valid versions found\nUse the acceptable-game-versions option to accept more game versions\nTo use datapacks, add a datapack loader mod and specify the datapack-folder option with the folder this mod loads datapacks from")
	}

	// TODO: option to always compare using flexver?
	// TODO: ask user which one to use?
	flexverLatest := findLatestVersion(result, gameVersions, true)
	releaseDateLatest := findLatestVersion(result, gameVersions, false)
	if flexverLatest != releaseDateLatest && releaseDateLatest.VersionNumber != nil && flexverLatest.VersionNumber != nil {
		fmt.Printf("Warning: Modrinth versions for %s inconsistent between latest version number and newest release date (%s vs %s)\n", name, *flexverLatest.VersionNumber, *releaseDateLatest.VersionNumber)
	}

	return releaseDateLatest, nil
}

func getSide(mod *modrinthApi.Project) string {
	server := shouldDownloadOnSide(*mod.ServerSide)
	client := shouldDownloadOnSide(*mod.ClientSide)

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

func getBestHash(v *modrinthApi.File) (string, string) {
	// Try preferred hashes first; SHA1 is first as it is required for Modrinth pack exporting
	val, exists := v.Hashes["sha1"]
	if exists {
		return "sha1", val
	}
	val, exists = v.Hashes["sha512"]
	if exists {
		return "sha512", val
	}
	val, exists = v.Hashes["sha256"]
	if exists {
		return "sha256", val
	}
	val, exists = v.Hashes["murmur2"] // (not defined in Modrinth pack spec, use with caution)
	if exists {
		return "murmur2", val
	}

	//none of the preferred hashes are present, just get the first one
	for key, val := range v.Hashes {
		return key, val
	}

	//No hashes were present
	return "", ""
}

func getInstalledProjectIDs(index *core.Index) []string {
	var installedProjects []string
	for _, modPath := range index.GetAllMods() {
		mod, err := core.LoadMod(modPath)
		if err == nil {
			data, ok := mod.GetParsedUpdateData("modrinth")
			if ok {
				updateData, ok := data.(mrUpdateData)
				if ok {
					if len(updateData.ProjectID) > 0 {
						installedProjects = append(installedProjects, updateData.ProjectID)
					}
				}
			}
		}
	}
	return installedProjects
}

func resolveVersion(project *modrinthApi.Project, version string) (*modrinthApi.Version, error) {
	// If it exists in the version list, it is already a version ID (and doesn't need querying further)
	if slices.Contains(project.Versions, version) {
		versionData, err := mrDefaultClient.Versions.Get(version)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch version %s: %v", version, err)
		}
		return versionData, nil
	}

	// Look up all versions
	// TODO: PR a version number filter to Modrinth?
	versionsList, err := mrDefaultClient.Versions.ListVersions(*project.ID, modrinthApi.ListVersionsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version list for %s: %v", *project.ID, err)
	}
	for _, v := range versionsList {
		if *v.VersionNumber == version {
			return v, nil
		}
	}
	return nil, fmt.Errorf("unable to find version %s", version)
}
