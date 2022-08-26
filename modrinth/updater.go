package modrinth

import (
	modrinthApi "codeberg.org/jmansfield/go-modrinth/modrinth"
	"errors"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/packwiz/packwiz/core"
)

type mrUpdateData struct {
	ModID            string `mapstructure:"mod-id"`
	InstalledVersion string `mapstructure:"version"`
}

func (u mrUpdateData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}

type mrUpdater struct{}

func (u mrUpdater) ParseUpdate(updateUnparsed map[string]interface{}) (interface{}, error) {
	var updateData mrUpdateData
	err := mapstructure.Decode(updateUnparsed, &updateData)
	return updateData, err
}

type cachedStateStore struct {
	ModID   string
	Version *modrinthApi.Version
}

func (u mrUpdater) CheckUpdate(mods []core.Mod, mcVersion string, pack core.Pack) ([]core.UpdateCheck, error) {
	results := make([]core.UpdateCheck, len(mods))

	for i, mod := range mods {
		rawData, ok := mod.GetParsedUpdateData("modrinth")
		if !ok {
			results[i] = core.UpdateCheck{Error: errors.New("couldn't parse mod data")}
			continue
		}

		data := rawData.(mrUpdateData)

		newVersion, err := getLatestVersion(data.ModID, pack)
		if err != nil {
			results[i] = core.UpdateCheck{Error: fmt.Errorf("failed to get latest version: %v", err)}
			continue
		}

		if *newVersion.ID == data.InstalledVersion { //The latest version from the site is the same as the installed one
			results[i] = core.UpdateCheck{UpdateAvailable: false}
			continue
		}

		if len(newVersion.Files) == 0 {
			results[i] = core.UpdateCheck{Error: errors.New("new version doesn't have any files")}
			continue
		}

		newFilename := newVersion.Files[0].Filename
		// Prefer the primary file
		for _, v := range newVersion.Files {
			if *v.Primary {
				newFilename = v.Filename
			}
		}

		results[i] = core.UpdateCheck{
			UpdateAvailable: true,
			UpdateString:    mod.FileName + " -> " + *newFilename,
			CachedState:     cachedStateStore{data.ModID, newVersion},
		}
	}

	return results, nil
}

func (u mrUpdater) DoUpdate(mods []*core.Mod, cachedState []interface{}) error {
	for i, mod := range mods {
		modState := cachedState[i].(cachedStateStore)
		var version = modState.Version

		var file = version.Files[0]
		// Prefer the primary file
		for _, v := range version.Files {
			if *v.Primary {
				file = v
			}
		}

		algorithm, hash := getBestHash(file)
		if algorithm == "" {
			return errors.New("file for mod " + mod.Name + " doesn't have a hash")
		}

		mod.FileName = *file.Filename
		mod.Download = core.ModDownload{
			URL:        *file.URL,
			HashFormat: algorithm,
			Hash:       hash,
		}
		mod.Update["modrinth"]["version"] = version.ID
	}

	return nil
}
