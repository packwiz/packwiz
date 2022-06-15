package github

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/packwiz/packwiz/core"
)

type ghUpdateData struct {
	ModID            string `mapstructure:"mod-id"` // The slug of the repo but named modId for consistency reasons
	InstalledVersion string `mapstructure:"version"`
	Branch           string `mapstructure:"branch"`
}

type ghUpdater struct{}

func (u ghUpdater) ParseUpdate(updateUnparsed map[string]interface{}) (interface{}, error) {
	var updateData ghUpdateData
	err := mapstructure.Decode(updateUnparsed, &updateData)
	return updateData, err
}

type cachedStateStore struct {
	ModID   string
	Version ModReleases
}

func (u ghUpdater) CheckUpdate(mods []core.Mod, mcVersion string, pack core.Pack) ([]core.UpdateCheck, error) {
	results := make([]core.UpdateCheck, len(mods))

	for i, mod := range mods {
		rawData, ok := mod.GetParsedUpdateData("github")
		if !ok {
			results[i] = core.UpdateCheck{Error: errors.New("couldn't parse mod data")}
			continue
		}

		data := rawData.(ghUpdateData)

		newVersion, err := getLatestVersion(data.ModID, pack, data.Branch)
		if err != nil {
			results[i] = core.UpdateCheck{Error: fmt.Errorf("failed to get latest version: %v", err)}
			continue
		}

		if newVersion.TagName == data.InstalledVersion { //The latest version from the site is the same as the installed one
			results[i] = core.UpdateCheck{UpdateAvailable: false}
			continue
		}

		if len(newVersion.Assets) == 0 {
			results[i] = core.UpdateCheck{Error: errors.New("new version doesn't have any files")}
			continue
		}

		newFilename := newVersion.Assets[0].Name

		results[i] = core.UpdateCheck{
			UpdateAvailable: true,
			UpdateString:    mod.FileName + " -> " + newFilename,
			CachedState:     cachedStateStore{data.ModID, newVersion},
		}
	}

	return results, nil
}

func (u ghUpdater) DoUpdate(mods []*core.Mod, cachedState []interface{}) error {
	for i, mod := range mods {
		modState := cachedState[i].(cachedStateStore)
		var version = modState.Version

		var file = version.Assets[0]
		for _, v := range version.Assets {
			if strings.HasSuffix(v.Name, ".jar") {
				file = v
			}
		}

		hash, error := file.getSha256()
		if error != nil || hash == "" {
			return errors.New("file doesn't have a hash")
		}

		mod.FileName = file.Name
		mod.Download = core.ModDownload{
			URL:        file.BrowserDownloadURL,
			HashFormat: "sha256",
			Hash:       hash,
		}
		mod.Update["github"]["version"] = version.ID
	}

	return nil
}
