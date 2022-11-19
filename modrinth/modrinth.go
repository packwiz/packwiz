package modrinth

import (
	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"errors"
	"github.com/packwiz/packwiz/cmd"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/unascribed/FlexVer/go/flexver"
	"golang.org/x/exp/slices"
	"net/http"
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

func getModIdsViaSearch(query string, versions []string) ([]*modrinthApi.SearchResult, error) {
	facets := make([]string, 0)
	for _, v := range versions {
		facets = append(facets, "versions:"+v)
	}

	res, err := mrDefaultClient.Projects.Search(&modrinthApi.SearchOptions{
		Limit: 5,
		Index: "relevance",
		// Filters by mod since currently only mods and modpacks are supported by Modrinth
		Facets: [][]string{facets, {"project_type:mod"}},
		Query:  query,
	})

	if err != nil {
		return nil, err
	}
	return res.Hits, nil
}

func getLatestVersion(modID string, pack core.Pack) (*modrinthApi.Version, error) {
	mcVersion, err := pack.GetMCVersion()
	if err != nil {
		return nil, err
	}
	gameVersions := append([]string{mcVersion}, viper.GetStringSlice("acceptable-game-versions")...)

	result, err := mrDefaultClient.Versions.ListVersions(modID, modrinthApi.ListVersionsOptions{
		GameVersions: gameVersions,
		Loaders:      pack.GetLoaders(),
	})

	if len(result) == 0 {
		return nil, errors.New("no valid versions found")
	}

	latestValidVersion := result[0]
	for _, v := range result[1:] {
		// Use FlexVer to compare versions
		compare := flexver.Compare(*v.VersionNumber, *latestValidVersion.VersionNumber)

		if compare == 0 {
			// Prefer Quilt over Fabric (Modrinth backend handles filtering)
			if slices.Contains(v.Loaders, "quilt") && !slices.Contains(latestValidVersion.Loaders, "quilt") {
				latestValidVersion = v
				continue
			}

			//Semver is equal, compare date instead
			if v.DatePublished.After(*latestValidVersion.DatePublished) {
				latestValidVersion = v
			}
		} else if compare > 0 {
			latestValidVersion = v
		}
	}

	return latestValidVersion, nil
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
					if len(updateData.ModID) > 0 {
						installedProjects = append(installedProjects, updateData.ModID)
					}
				}
			}
		}
	}
	return installedProjects
}
