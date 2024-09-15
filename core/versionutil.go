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

type ModLoaderComponent struct {
	Name              string
	FriendlyName      string
	VersionListGetter func(mcVersion string) ([]string, string, error)
}

var ModLoaders = map[string]ModLoaderComponent{
	"fabric": {
		// There's no need to specify yarn version - yarn isn't used outside a dev environment, and intermediary corresponds to game version anyway
		Name:              "fabric",
		FriendlyName:      "Fabric loader",
		VersionListGetter: FetchMavenVersionList("https://maven.fabricmc.net/net/fabricmc/fabric-loader/maven-metadata.xml"),
	},
	"forge": {
		Name:              "forge",
		FriendlyName:      "Forge",
		VersionListGetter: FetchMavenVersionPrefixedListStrip("https://files.minecraftforge.net/maven/net/minecraftforge/forge/maven-metadata.xml", "Forge"),
	},
	"liteloader": {
		Name:              "liteloader",
		FriendlyName:      "LiteLoader",
		VersionListGetter: FetchMavenVersionPrefixedList("https://repo.mumfrey.com/content/repositories/snapshots/com/mumfrey/liteloader/maven-metadata.xml", "LiteLoader"),
	},
	"quilt": {
		Name:              "quilt",
		FriendlyName:      "Quilt loader",
		VersionListGetter: FetchMavenVersionList("https://maven.quiltmc.org/repository/release/org/quiltmc/quilt-loader/maven-metadata.xml"),
	},
	"neoforge": {
		Name:              "neoforge",
		FriendlyName:      "NeoForge",
		VersionListGetter: FetchMavenVersionList("https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml"),
	},
}

func FetchMavenVersionList(url string) func(mcVersion string) ([]string, string, error) {
	return func(mcVersion string) ([]string, string, error) {
		res, err := GetWithUA(url, "application/xml")
		if err != nil {
			return []string{}, "", err
		}
		dec := xml.NewDecoder(res.Body)
		out := MavenMetadata{}
		err = dec.Decode(&out)
		if err != nil {
			return []string{}, "", err
		}
		return out.Versioning.Versions.Version, out.Versioning.Release, nil
	}
}

func FetchMavenVersionPrefixedList(url string, friendlyName string) func(mcVersion string) ([]string, string, error) {
	return func(mcVersion string) ([]string, string, error) {
		res, err := GetWithUA(url, "application/xml")
		if err != nil {
			return []string{}, "", err
		}
		dec := xml.NewDecoder(res.Body)
		out := MavenMetadata{}
		err = dec.Decode(&out)
		if err != nil {
			return []string{}, "", err
		}
		allowedVersions := make([]string, 0, len(out.Versioning.Versions.Version))
		for _, v := range out.Versioning.Versions.Version {
			if hasPrefixSplitDash(v, mcVersion) {
				allowedVersions = append(allowedVersions, v)
			}
		}
		if len(allowedVersions) == 0 {
			return []string{}, "", errors.New("no " + friendlyName + " versions available for this Minecraft version")
		}
		if hasPrefixSplitDash(out.Versioning.Release, mcVersion) {
			return allowedVersions, out.Versioning.Release, nil
		}
		if hasPrefixSplitDash(out.Versioning.Latest, mcVersion) {
			return allowedVersions, out.Versioning.Latest, nil
		}
		// Sort list to get largest version
		flexver.VersionSlice(allowedVersions).Sort()
		return allowedVersions, allowedVersions[len(allowedVersions)-1], nil
	}
}

func FetchMavenVersionPrefixedListStrip(url string, friendlyName string) func(mcVersion string) ([]string, string, error) {
	noStrip := FetchMavenVersionPrefixedList(url, friendlyName)
	return func(mcVersion string) ([]string, string, error) {
		versions, latestVersion, err := noStrip(mcVersion)
		if err != nil {
			return nil, "", err
		}
		for k, v := range versions {
			versions[k] = removeMcVersion(v, mcVersion)
		}
		latestVersion = removeMcVersion(latestVersion, mcVersion)
		return versions, latestVersion, nil
	}
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
