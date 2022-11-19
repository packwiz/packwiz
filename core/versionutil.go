package core

import (
	"encoding/xml"
	"errors"
	"github.com/unascribed/FlexVer/go/flexver"
	"strings"
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
		flexver.VersionSlice(out.Versioning.Versions.Version).Sort()
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
