package core

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/unascribed/FlexVer/go/flexver"
)

type MavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Versioning struct {
		Release  string `xml:"release"`
		Latest   string `xml:"latest"`
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
		LastUpdated string `xml:"lastUpdated"`
	} `xml:"versioning"`
}

type ModLoaderVersions struct {
	// All Versions of this modloader
	Versions []string
	// The Latest/preferred version for this modloader
	Latest string
}

type ModLoaderComponent struct {
	// An identifier for the modloader
	Name string
	// A user-friendly name
	FriendlyName string
	// Retrieves the list of all modloader versions. Modloader versions are always filtered to those compatible
	// with a specific minecraft version.
	VersionListGetter func(q VersionListQuery) (*ModLoaderVersions, error)
}

var modLoadersList = []ModLoaderComponent{
	{
		// There's no need to specify yarn version - yarn isn't used outside a dev environment, and intermediary corresponds to game version anyway
		Name:         "fabric",
		FriendlyName: "Fabric loader",
		VersionListGetter: func(q VersionListQuery) (*ModLoaderVersions, error) {
			// Fabric loaders isn't locked to a mc version per se
			return fetchVersionsFromMaven(q, "https://maven.fabricmc.net/net/fabricmc/fabric-loader/maven-metadata.xml")
		},
	},
	{
		Name:              "forge",
		FriendlyName:      "Forge",
		VersionListGetter: fetchForForge,
	},
	{
		Name:         "liteloader",
		FriendlyName: "LiteLoader",
		VersionListGetter: func(q VersionListQuery) (*ModLoaderVersions, error) {
			return fetchLiteloaderStyle(q, "https://repo.mumfrey.com/content/repositories/snapshots/com/mumfrey/liteloader/maven-metadata.xml")
		},
	},
	{
		Name:         "quilt",
		FriendlyName: "Quilt loader",
		VersionListGetter: func(q VersionListQuery) (*ModLoaderVersions, error) {
			return fetchVersionsFromMaven(q, "https://maven.quiltmc.org/repository/release/org/quiltmc/quilt-loader/maven-metadata.xml")
		},
	},
	{
		Name:              "neoforge",
		FriendlyName:      "NeoForge",
		VersionListGetter: fetchForNeoForge,
	},
}

// A map containing information about all supported modloaders.
// Can be indexed by the [ModLoaderComponent]'s name, which serves as an identifier.
var ModLoaders = createModloaderMap(modLoadersList)

func createModloaderMap(input []ModLoaderComponent) map[string]ModLoaderComponent {
	var mlMap = make(map[string]ModLoaderComponent)
	for _, loader := range input {
		mlMap[loader.Name] = loader
	}
	return mlMap
}

type QueryType int

const (
	// The Latest field will contain the last released loader version
	Latest QueryType = iota
	// The Latest field will contain the loader version recommended for use
	Recommended
)

type VersionListQuery struct {
	// Which loader to query versions for
	Loader ModLoaderComponent
	// Which minecraft version the returned loader versions should be compatible with
	McVersion string
	// Determines how the latest version is determined
	QueryType QueryType
}

func MakeQuery(loader ModLoaderComponent, mcVersion string) VersionListQuery {
	return VersionListQuery{
		Loader:    loader,
		McVersion: mcVersion,
		QueryType: Latest,
	}
}

func (in VersionListQuery) WithQueryType(queryType QueryType) VersionListQuery {
	return VersionListQuery{
		Loader:    in.Loader,
		McVersion: in.McVersion,
		QueryType: queryType,
	}
}

// Queries the versions of a modloader
func DoQuery(q VersionListQuery) (*ModLoaderVersions, error) {
	return q.Loader.VersionListGetter(q)
}

// Retrieve a list of versions from maven, with no filtering or processing of the maven data
func fetchVersionsFromMaven(q VersionListQuery, url string) (*ModLoaderVersions, error) {
	identity_function := func(version string) *string { return &version }
	return fetchMavenWithFilterMap(q, url, identity_function)
}

func fetchForgeStyle(q VersionListQuery, url string) (*ModLoaderVersions, error) {
	// Forge style:
	// each version is formatted like `mcVersion-forgeVersion`
	// eg: `1.18.1-39.0.18`
	return fetchMavenWithFilterMap(q, url, func(version string) *string {
		before, after, f := strings.Cut(version, "-")
		if !f {
			// The version didn't have a dash? Lets just reject it entirely
			return nil
		}
		if before != q.McVersion {
			// The part before the dash should match the mc version we're looking for
			return nil
		}
		// The part after the dash is the actual version, and the part we care about
		return &after
	})
}

func fetchNeoForgeStyle(q VersionListQuery, url string) (*ModLoaderVersions, error) {
	// NeoForge style:
	// If minecraft versions are in the form of 1.a.b, then neoforge versions are in the form of a.b.x
	// Eg, for minecraft 1.20.6, neoforge version 20.6.2 and 20.6.83-beta would both be valid versions
	// for minecraft 1.20.2, neoforge version 20.2.23-beta
	// for minecraft 1.21, neoforge version 21.0.143 would be valid

	var mcSplit = strings.Split(q.McVersion, ".")
	if len(mcSplit) < 2 {
		// This does not appear to be a minecraft version that's formatted in a way that matches neoforge
		return nil, fmt.Errorf("packwiz cannot detect compatible %s versions for this Minecraft version (%s)", q.Loader.FriendlyName, q.McVersion)
	}
	var mcMajor = mcSplit[1]
	var mcMinor = "0"
	if len(mcSplit) > 2 {
		mcMinor = mcSplit[2]
	}
	// Note that the period at the end is significant, we don't want to match `21.10.43` as being for 1.21.1 (instead of 1.21.10)
	var requiredPrefix = mcMajor + "." + mcMinor + "."

	return fetchMavenWithFilterMap(q, url, func(version string) *string {
		if !strings.HasPrefix(version, requiredPrefix) {
			// Reject NeoForge versions that don't have the right prefix for this mc version
			return nil
		}
		return &version
	})
}

func fetchLiteloaderStyle(q VersionListQuery, url string) (*ModLoaderVersions, error) {
	// Liteloader style:
	// each version is formatted like `mcVersion-SNAPSHOT`
	// eg: `1.12.2-SNAPSHOT`
	// (yes, it appears like liteloader only has a single version per mc version)
	return fetchMavenWithFilterMap(q, url, func(version string) *string {
		before, _, f := strings.Cut(version, "-")
		// Check if the part before the dash matches the mc version we're looking for
		if f && before == q.McVersion {
			return &version
		} else {
			return nil
		}
	})
}

// Retrieves all versions through maven metadata, and then processes the using the provided `filterMap` function.
// When `filterMap` returns a string, the version will be renamed to the provided string. If `nil` is returned, the
// version is marked as invalid and will not be considered in the result.
func fetchMavenWithFilterMap(q VersionListQuery, url string, filterMap func(version string) *string) (*ModLoaderVersions, error) {
	res, err := GetWithUA(url, "application/xml")
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(res.Body)
	out := MavenMetadata{}
	err = dec.Decode(&out)
	if err != nil {
		return nil, err
	}

	// Pass all of the versions listed in the maven through our filterMap
	versions := make([]string, 0, len(out.Versioning.Versions.Version))
	for _, v := range out.Versioning.Versions.Version {
		mappedV := filterMap(v)
		if mappedV != nil {
			versions = append(versions, *mappedV)
		}
	}

	if len(versions) == 0 {
		return nil, errors.New("no " + q.Loader.FriendlyName + " versions available for " + q.McVersion)
	}

	// Determine the latest release
	var latestRelease = ""
	release := filterMap(out.Versioning.Release)
	latest := filterMap(out.Versioning.Latest)
	if release != nil {
		latestRelease = *release
	} else if latest != nil {
		latestRelease = *latest
	} else {
		// Maven was useless, just rely on flexver sorting
		flexver.VersionSlice(versions).Sort()
		latestRelease = versions[len(versions)-1]
	}
	return &ModLoaderVersions{versions, latestRelease}, nil
}

func fetchForNeoForge(q VersionListQuery) (*ModLoaderVersions, error) {
	// NeoForge reused Forge's versioning scheme for 1.20.1, but moved to their own versioning scheme for 1.20.2 and above
	if q.McVersion == "1.20.1" {
		return fetchForgeStyle(q, "https://maven.neoforged.net/releases/net/neoforged/forge/maven-metadata.xml")
	} else {
		return fetchNeoForgeStyle(q, "https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml")
	}
}

func fetchForForge(q VersionListQuery) (*ModLoaderVersions, error) {
	result, err := fetchForgeStyle(q, "https://files.minecraftforge.net/maven/net/minecraftforge/forge/maven-metadata.xml")
	if err != nil {
		return nil, err
	}
	// Forge is the only loader which defines a special recommended version
	if q.QueryType == Recommended {
		recommended := getForgeRecommended(q)
		if recommended != "" {
			result.Latest = recommended
		}
	}
	return result, nil
}

func ComponentToFriendlyName(component string) string {
	if component == "minecraft" {
		return "Minecraft"
	}
	loader, ok := ModLoaders[component]
	if ok {
		return loader.FriendlyName
	} else {
		return component
	}
}

// HighestSliceIndex returns the highest index of the given values in the slice (-1 if no value is found in the slice)
func HighestSliceIndex(slice []string, values []string) int {
	highest := -1
	for _, val := range values {
		for i, v := range slice {
			if v == val && i > highest {
				highest = i
			}
		}
	}
	return highest
}

type ForgeRecommended struct {
	Homepage string            `json:"homepage"`
	Versions map[string]string `json:"promos"`
}

// getForgeRecommended gets the recommended version of Forge for the given Minecraft version
func getForgeRecommended(q VersionListQuery) string {
	res, err := GetWithUA("https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json", "application/json")
	if err != nil {
		return ""
	}
	dec := json.NewDecoder(res.Body)
	out := ForgeRecommended{}
	err = dec.Decode(&out)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	// Get mcVersion-recommended, if it doesn't exist then get mcVersion-latest
	// If neither exist, return empty string
	recommendedString := fmt.Sprintf("%s-recommended", q.McVersion)
	if out.Versions[recommendedString] != "" {
		return out.Versions[recommendedString]
	}
	latestString := fmt.Sprintf("%s-latest", q.McVersion)
	if out.Versions[latestString] != "" {
		return out.Versions[latestString]
	}
	return ""
}
