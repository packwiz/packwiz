package modrinth

import (
    "errors"

	"github.com/comp500/packwiz/core"
	"github.com/mitchellh/mapstructure"
)

type mrUpdateData struct {
	ModID               string  `mapstructure:"mod-id"`
	Versions            int     `mapstructure:"versions"`
	InstalledVersion    string  `mapstructure:"installed"`
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
    Mod Mod
    Version Version
}

func (u mrUpdater) CheckUpdate(mods []core.Mod, mcVersion string) ([]core.UpdateCheck, error) {
    results := make([]core.UpdateCheck, len(mods))

    for i, mod := range mods {
        rawData, ok := mod.GetParsedUpdateData("modrinth")
        if !ok {
            results[i] = core.UpdateCheck{Error: errors.New("couldn't parse mod data")}
            continue
        }

        data := rawData.(mrUpdateData)

        fetchedMod, err := fetchMod(data.ModID)
        if err != nil {
            results[i] = core.UpdateCheck{Error: err}
            continue
        }

        if len(fetchedMod.Versions) == data.Versions {
            //the amount of listed versions hasn't changed. There shouldn't be any update
            results[i] = core.UpdateCheck{UpdateAvailable: false}
            continue
        }

        latestVersion, err := fetchedMod.fetchAndGetLatestVersion(mcVersion);
        if err != nil {
            results[i] = core.UpdateCheck{Error: err}
            continue
        }

        if latestVersion.ID == data.InstalledVersion {
            results[i] = core.UpdateCheck{UpdateAvailable: false}
            continue
        }

        if len(latestVersion.Files) == 0 {
            results[i] = core.UpdateCheck{Error: errors.New("new version doesn't have any files")}
            continue
        }

        results[i] = core.UpdateCheck{
            UpdateAvailable: true,
            UpdateString:    mod.FileName + " -> " + latestVersion.Files[0].Filename,
            CachedState:     cachedStateStore{fetchedMod, latestVersion},
        }
    }

    return results, nil
}

func (u mrUpdater) DoUpdate(mods []*core.Mod, cachedState []interface{}) error {
    pack, err := core.LoadPack()
    if err != nil {
        return err
    }

    for i, _ := range mods {
        modState := cachedState[i].(cachedStateStore)

        err := installVersion(modState.Mod, modState.Version, pack)
        if err != nil {
            return err
        }
    }

    return nil
}