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
	VersionListGetter func(mcVersion string) (*ModLoaderVersions, error)
}

var modLoadersList = []ModLoaderComponent{
	{
		// There's no need to specify yarn version - yarn isn't used outside a dev environment, and intermediary corresponds to game version anyway
		Name:         "fabric",
		FriendlyName: "Fabric loader",
		VersionListGetter: func(mcVersion string) (*ModLoaderVersions, error) {
			// Fabric loaders isn't locked to a mc version per se
			return FetchMavenVersionList("https://maven.fabricmc.net/net/fabricmc/fabric-loader/maven-metadata.xml")
		},
	},
	{
		Name:         "forge",
		FriendlyName: "Forge",
		VersionListGetter: func(mcVersion string) (*ModLoaderVersions, error) {
			return FetchMavenVersionPrefixedListStrip("https://files.minecraftforge.net/maven/net/minecraftforge/forge/maven-metadata.xml", "Forge", mcVersion)
		},
	},
	{
		Name:         "liteloader",
		FriendlyName: "LiteLoader",
		VersionListGetter: func(mcVersion string) (*ModLoaderVersions, error) {
			return FetchMavenVersionPrefixedList("https://repo.mumfrey.com/content/repositories/snapshots/com/mumfrey/liteloader/maven-metadata.xml", "LiteLoader", mcVersion)
		},
	},
	{
		Name:         "quilt",
		FriendlyName: "Quilt loader",
		VersionListGetter: func(mcVersion string) (*ModLoaderVersions, error) {
			return FetchMavenVersionList("https://maven.quiltmc.org/repository/release/org/quiltmc/quilt-loader/maven-metadata.xml")
		},
	},
	{
		Name:         "neoforge",
		FriendlyName: "NeoForge",
		VersionListGetter: func(mcVersion string) (*ModLoaderVersions, error) {
			return FetchNeoForge(mcVersion)
		},
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

func FetchMavenVersionList(url string) (*ModLoaderVersions, error) {
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
	return &ModLoaderVersions{out.Versioning.Versions.Version, out.Versioning.Release}, nil
}

func FetchMavenVersionFiltered(url string, friendlyName string, mcVersion string, filter func(version string, mcVersion string) bool) (*ModLoaderVersions, error) {
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
	allowedVersions := make([]string, 0, len(out.Versioning.Versions.Version))
	for _, v := range out.Versioning.Versions.Version {
		if filter(v, mcVersion) {
			allowedVersions = append(allowedVersions, v)
		}
	}
	if len(allowedVersions) == 0 {
		return nil, errors.New("no " + friendlyName + " versions available for this Minecraft version")
	}
	if filter(out.Versioning.Release, mcVersion) {
		return &ModLoaderVersions{allowedVersions, out.Versioning.Release}, nil
	}
	if filter(out.Versioning.Latest, mcVersion) {
		return &ModLoaderVersions{allowedVersions, out.Versioning.Latest}, nil
	}
	// Sort list to get largest version
	flexver.VersionSlice(allowedVersions).Sort()
	return &ModLoaderVersions{allowedVersions, allowedVersions[len(allowedVersions)-1]}, nil
}

func FetchMavenVersionPrefixedList(url string, friendlyName string, mcVersion string) (*ModLoaderVersions, error) {
	return FetchMavenVersionFiltered(url, friendlyName, mcVersion, hasPrefixSplitDash)
}

func FetchMavenVersionPrefixedListStrip(url string, friendlyName string, mcVersion string) (*ModLoaderVersions, error) {
	versionData, err := FetchMavenVersionPrefixedList(url, friendlyName, mcVersion)
	if err != nil {
		return nil, err
	}
	// Perform stripping on all the results
	for k, v := range versionData.Versions {
		versionData.Versions[k] = removeMcVersion(v, mcVersion)
	}
	versionData.Latest = removeMcVersion(versionData.Latest, mcVersion)
	return versionData, nil
}

func removeMcVersion(str string, mcVersion string) string {
	components := strings.Split(str, "-")
	newComponents := make([]string, 0)
	for _, v := range components {
		if v != mcVersion {
			newComponents = append(newComponents, v)
		}
	}
	return strings.Join(newComponents, "-")
}

func hasPrefixSplitDash(str string, prefix string) bool {
	components := strings.Split(str, "-")
	if len(components) > 1 && components[0] == prefix {
		return true
	}
	return false
}

func FetchNeoForge(mcVersion string) (*ModLoaderVersions, error) {
	// NeoForge reused Forge's versioning scheme for 1.20.1, but moved to their own versioning scheme for 1.20.2 and above
	if mcVersion == "1.20.1" {
		return FetchMavenVersionPrefixedListStrip("https://maven.neoforged.net/releases/net/neoforged/forge/maven-metadata.xml", "NeoForge", mcVersion)
	} else {
		return FetchMavenWithNeoForgeStyleVersions("https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml", "NeoForge", mcVersion)
	}
}

func FetchMavenWithNeoForgeStyleVersions(url string, friendlyName string, mcVersion string) (*ModLoaderVersions, error) {
	return FetchMavenVersionFiltered(url, friendlyName, mcVersion, func(neoforgeVersion string, mcVersion string) bool {
		// Minecraft versions are in the form of 1.a.b
		// Neoforge versions are in the form of a.b.x
		// Eg, for minecraft 1.20.6, neoforge version 20.6.2 and 20.6.83-beta would both be valid versions
		// for minecraft 1.20.2, neoforge version 20.2.23-beta
		// for minecraft 1.21, neoforge version 21.0.143 would be valid
		var mcSplit = strings.Split(mcVersion, ".")
		if len(mcSplit) < 2 {
			// This does not appear to be a minecraft version that's formatted in a way that matches neoforge
			return false
		}
		var mcMajor = mcSplit[1]
		var mcMinor = "0"
		if len(mcSplit) > 2 {
			mcMinor = mcSplit[2]
		}

		// We can only accept an exact version number match, because otherwise 1.21.1 would pull in loader versions for 1.21.10.
		var requiredPrefix = mcMajor + "." + mcMinor + "."

		return strings.HasPrefix(neoforgeVersion, requiredPrefix)
	})
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

// GetForgeRecommended gets the recommended version of Forge for the given Minecraft version
func GetForgeRecommended(mcVersion string) string {
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
	recommendedString := fmt.Sprintf("%s-recommended", mcVersion)
	if out.Versions[recommendedString] != "" {
		return out.Versions[recommendedString]
	}
	latestString := fmt.Sprintf("%s-latest", mcVersion)
	if out.Versions[latestString] != "" {
		return out.Versions[latestString]
	}
	return ""
}
