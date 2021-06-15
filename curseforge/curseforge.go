package curseforge

import (
	"errors"
	"github.com/spf13/viper"
	"regexp"
	"strconv"
	"strings"

	"github.com/comp500/packwiz/cmd"
	"github.com/comp500/packwiz/core"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
)

var curseforgeCmd = &cobra.Command{
	Use:     "curseforge",
	Aliases: []string{"cf", "curse"},
	Short:   "Manage curseforge-based mods",
}

func init() {
	cmd.Add(curseforgeCmd)
	core.Updaters["curseforge"] = cfUpdater{}
}

var fileIDRegexes = [...]*regexp.Regexp{
	regexp.MustCompile("^https?://minecraft\\.curseforge\\.com/projects/(.+)/files/(\\d+)"),
	regexp.MustCompile("^https?://(?:www\\.)?curseforge\\.com/minecraft/mc-mods/(.+)/files/(\\d+)"),
	regexp.MustCompile("^https?://(?:www\\.)?curseforge\\.com/minecraft/mc-mods/(.+)/download/(\\d+)"),
}

var snapshotVersionRegex = regexp.MustCompile("(?:Snapshot )?(\\d+)w0?(0|[1-9]\\d*)([a-z])")

var snapshotNames = [...]string{"-pre", " Pre-Release ", " Pre-release ", "-rc"}

func getCurseforgeVersion(mcVersion string) string {
	for _, name := range snapshotNames {
		index := strings.Index(mcVersion, name)
		if index > -1 {
			return mcVersion[:index] + "-Snapshot"
		}
	}

	matches := snapshotVersionRegex.FindStringSubmatch(mcVersion)
	if matches == nil {
		return mcVersion
	}
	year, err := strconv.Atoi(matches[1])
	if err != nil {
		return mcVersion
	}
	week, err := strconv.Atoi(matches[2])
	if err != nil {
		return mcVersion
	}

	if year == 20 && week >= 45 || year >= 21 {
		return "1.17-Snapshot"
	} else if year == 20 && week >= 6 {
		return "1.16-Snapshot"
	} else if year == 19 && week >= 34 {
		return "1.15-Snapshot"
	} else if year == 18 && week >= 43 || year == 19 && week <= 14 {
		return "1.14-Snapshot"
	} else if year == 18 && week >= 30 && week <= 33 {
		return "1.13.1-Snapshot"
	} else if year == 17 && week >= 43 || year == 18 && week <= 22 {
		return "1.13-Snapshot"
	} else if year == 17 && week == 31 {
		return "1.12.1-Snapshot"
	} else if year == 17 && week >= 6 && week <= 18 {
		return "1.12-Snapshot"
	} else if year == 16 && week == 50 {
		return "1.11.1-Snapshot"
	} else if year == 16 && week >= 32 && week <= 44 {
		return "1.11-Snapshot"
	} else if year == 16 && week >= 20 && week <= 21 {
		return "1.10-Snapshot"
	} else if year == 16 && week >= 14 && week <= 15 {
		return "1.9.3-Snapshot"
	} else if year == 15 && week >= 31 || year == 16 && week <= 7 {
		return "1.9-Snapshot"
	} else if year == 14 && week >= 2 && week <= 34 {
		return "1.8-Snapshot"
	} else if year == 13 && week >= 47 && week <= 49 {
		return "1.7.4-Snapshot"
	} else if year == 13 && week >= 36 && week <= 43 {
		return "1.7.2-Snapshot"
	} else if year == 13 && week >= 16 && week <= 26 {
		return "1.6-Snapshot"
	} else if year == 13 && week >= 11 && week <= 12 {
		return "1.5.1-Snapshot"
	} else if year == 13 && week >= 1 && week <= 10 {
		return "1.5-Snapshot"
	} else if year == 12 && week >= 49 && week <= 50 {
		return "1.4.6-Snapshot"
	} else if year == 12 && week >= 32 && week <= 42 {
		return "1.4.2-Snapshot"
	} else if year == 12 && week >= 15 && week <= 30 {
		return "1.3.1-Snapshot"
	} else if year == 12 && week >= 3 && week <= 8 {
		return "1.2.1-Snapshot"
	} else if year == 11 && week >= 47 || year == 12 && week <= 1 {
		return "1.1-Snapshot"
	}
	return mcVersion
}

func getFileIDsFromString(mod string) (bool, int, int, error) {
	for _, v := range fileIDRegexes {
		matches := v.FindStringSubmatch(mod)
		if matches != nil && len(matches) == 3 {
			modID, err := modIDFromSlug(matches[1])
			if err != nil {
				return true, 0, 0, err
			}
			fileID, err := strconv.Atoi(matches[2])
			if err != nil {
				return true, 0, 0, err
			}
			return true, modID, fileID, nil
		}
	}
	return false, 0, 0, nil
}

var modSlugRegexes = [...]*regexp.Regexp{
	regexp.MustCompile("^https?://minecraft\\.curseforge\\.com/projects/([^/]+)"),
	regexp.MustCompile("^https?://(?:www\\.)?curseforge\\.com/minecraft/mc-mods/([^/]+)"),
	// Exact slug matcher
	regexp.MustCompile("^[a-z][\\da-z\\-_]{0,127}$"),
}

func getModIDFromString(mod string) (bool, int, error) {
	// Check if it's just a number first
	modID, err := strconv.Atoi(mod)
	if err == nil && modID > 0 {
		return true, modID, nil
	}

	for _, v := range modSlugRegexes {
		matches := v.FindStringSubmatch(mod)
		if matches != nil {
			var slug string
			if len(matches) == 2 {
				slug = matches[1]
			} else if len(matches) == 1 {
				slug = matches[0]
			} else {
				continue
			}
			modID, err := modIDFromSlug(slug)
			if err != nil {
				return true, 0, err
			}
			return true, modID, nil
		}
	}
	return false, 0, nil
}

func createModFile(modInfo modInfo, fileInfo modFileInfo, index *core.Index) error {
	updateMap := make(map[string]map[string]interface{})
	var err error

	updateMap["curseforge"], err = cfUpdateData{
		ProjectID: modInfo.ID,
		FileID:    fileInfo.ID,
		// TODO: determine update channel
		ReleaseChannel: "beta",
	}.ToMap()
	if err != nil {
		return err
	}

	modMeta := core.Mod{
		Name:     modInfo.Name,
		FileName: fileInfo.FileName,
		Side:     core.UniversalSide,
		Download: core.ModDownload{
			URL: fileInfo.DownloadURL,
			// TODO: murmur2 hashing may be unstable in curse api, calculate the hash manually?
			// TODO: check if the hash is invalid (e.g. 0)
			HashFormat: "murmur2",
			Hash:       strconv.Itoa(fileInfo.Fingerprint),
		},
		Update: updateMap,
	}
	path := modMeta.SetMetaName(modInfo.Slug, *index)

	// If the file already exists, this will overwrite it!!!
	// TODO: Should this be improved?
	// Current strategy is to go ahead and do stuff without asking, with the assumption that you are using
	// VCS anyway.

	format, hash, err := modMeta.Write()
	if err != nil {
		return err
	}

	return index.RefreshFileWithHash(path, format, hash, true)
}

func getLoader(pack core.Pack) int {
	dependencies := pack.Versions

	_, hasFabric := dependencies["fabric"]
	_, hasForge := dependencies["forge"]
	if hasFabric && hasForge {
		return modloaderTypeAny
	} else if hasFabric {
		return modloaderTypeFabric
	} else if hasForge {
		return modloaderTypeForge
	} else {
		return modloaderTypeAny
	}
}

func matchLoaderType(packLoaderType int, modLoaderType int) bool {
	if packLoaderType == modloaderTypeAny || modLoaderType == modloaderTypeAny {
		return true
	} else {
		return packLoaderType == modLoaderType
	}
}

func matchLoaderTypeFileInfo(packLoaderType int, fileInfoData modFileInfo) bool {
	if packLoaderType == modloaderTypeAny {
		return true
	} else {
		if packLoaderType == modloaderTypeFabric {
			for _, v := range fileInfoData.GameVersions {
				if v == "Fabric" {
					return true
				}
			}
		} else if packLoaderType == modloaderTypeForge {
			for _, v := range fileInfoData.GameVersions {
				if v == "Forge" {
					return true
				}
			}
		} else {
			return true
		}
		return false
	}
}

func matchGameVersion(mcVersion string, modMcVersion string) bool {
	if getCurseforgeVersion(mcVersion) == modMcVersion {
		return true
	} else {
		for _, v := range viper.GetStringSlice("acceptable-game-versions") {
			if getCurseforgeVersion(v) == modMcVersion {
				return true
			}
		}
		return false
	}
}

func matchGameVersions(mcVersion string, modMcVersions []string) bool {
	for _, modMcVersion := range modMcVersions {
		if getCurseforgeVersion(mcVersion) == modMcVersion {
			return true
		} else {
			for _, v := range viper.GetStringSlice("acceptable-game-versions") {
				if getCurseforgeVersion(v) == modMcVersion {
					return true
				}
			}
		}
	}
	return false
}

type cfUpdateData struct {
	ProjectID      int    `mapstructure:"project-id"`
	FileID         int    `mapstructure:"file-id"`
	ReleaseChannel string `mapstructure:"release-channel"`
}

func (u cfUpdateData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(u, &newMap)
	return newMap, err
}

type cfUpdater struct{}

func (u cfUpdater) ParseUpdate(updateUnparsed map[string]interface{}) (interface{}, error) {
	var updateData cfUpdateData
	err := mapstructure.Decode(updateUnparsed, &updateData)
	return updateData, err
}

type cachedStateStore struct {
	modInfo
	hasFileInfo bool
	fileID      int
	fileInfo    modFileInfo
}

func (u cfUpdater) CheckUpdate(mods []core.Mod, mcVersion string, pack core.Pack) ([]core.UpdateCheck, error) {
	results := make([]core.UpdateCheck, len(mods))
	modIDs := make([]int, len(mods))
	modInfos := make([]modInfo, len(mods))

	for i, v := range mods {
		projectRaw, ok := v.GetParsedUpdateData("curseforge")
		if !ok {
			results[i] = core.UpdateCheck{Error: errors.New("couldn't parse mod data")}
			continue
		}
		project := projectRaw.(cfUpdateData)
		modIDs[i] = project.ProjectID
	}

	modInfosUnsorted, err := getModInfoMultiple(modIDs)
	if err != nil {
		return nil, err
	}
	for _, v := range modInfosUnsorted {
		for i, id := range modIDs {
			if id == v.ID {
				modInfos[i] = v
				break
			}
		}
	}

	packLoaderType := getLoader(pack)

	for i, v := range mods {
		projectRaw, ok := v.GetParsedUpdateData("curseforge")
		if !ok {
			results[i] = core.UpdateCheck{Error: errors.New("couldn't parse mod data")}
			continue
		}
		project := projectRaw.(cfUpdateData)

		updateAvailable := false
		fileID := project.FileID
		fileInfoObtained := false
		var fileInfoData modFileInfo
		var fileName string

		// For snapshots, curseforge doesn't put them in GameVersionLatestFiles
		for _, v := range modInfos[i].LatestFiles {
			// Choose "newest" version by largest ID
			if matchGameVersions(mcVersion, v.GameVersions) && v.ID > fileID && matchLoaderTypeFileInfo(packLoaderType, v) {
				updateAvailable = true
				fileID = v.ID
				fileInfoData = v
				fileInfoObtained = true
				fileName = v.FileName
			}
		}

		for _, file := range modInfos[i].GameVersionLatestFiles {
			// TODO: change to timestamp-based comparison??
			// TODO: manage alpha/beta/release correctly, check update channel?
			// Choose "newest" version by largest ID
			if matchGameVersion(mcVersion, file.GameVersion) && file.ID > fileID && matchLoaderType(packLoaderType, file.Modloader) {
				updateAvailable = true
				fileID = file.ID
				fileName = file.Name
				fileInfoObtained = false // Make sure we get the file info again
			}
		}

		if !updateAvailable {
			results[i] = core.UpdateCheck{UpdateAvailable: false}
			continue
		}

		// The API also provides some files inline, because that's efficient!
		if !fileInfoObtained {
			for _, file := range modInfos[i].LatestFiles {
				if file.ID == fileID {
					fileInfoObtained = true
					fileInfoData = file
				}
			}
		}

		results[i] = core.UpdateCheck{
			UpdateAvailable: true,
			UpdateString:    v.FileName + " -> " + fileName,
			CachedState:     cachedStateStore{modInfos[i], fileInfoObtained, fileID, fileInfoData},
		}
	}
	return results, nil
}

func (u cfUpdater) DoUpdate(mods []*core.Mod, cachedState []interface{}) error {
	// "Do" isn't really that accurate, more like "Apply", because all the work is done in CheckUpdate!
	for i, v := range mods {
		modState := cachedState[i].(cachedStateStore)

		fileInfoData := modState.fileInfo
		if !modState.hasFileInfo {
			var err error
			fileInfoData, err = getFileInfo(modState.ID, modState.fileID)
			if err != nil {
				return err
			}
		}

		v.FileName = fileInfoData.FileName
		v.Name = modState.Name
		v.Download = core.ModDownload{
			URL: fileInfoData.DownloadURL,
			// TODO: murmur2 hashing may be unstable in curse api, calculate the hash manually?
			// TODO: check if the hash is invalid (e.g. 0)
			HashFormat: "murmur2",
			Hash:       strconv.Itoa(fileInfoData.Fingerprint),
		}

		v.Update["curseforge"]["project-id"] = modState.ID
		v.Update["curseforge"]["file-id"] = fileInfoData.ID
	}

	return nil
}

type cfExportData struct {
	ProjectID int `mapstructure:"project-id"`
}

func (e cfExportData) ToMap() (map[string]interface{}, error) {
	newMap := make(map[string]interface{})
	err := mapstructure.Decode(e, &newMap)
	return newMap, err
}

func parseExportData(from map[string]interface{}) (cfExportData, error) {
	var exportData cfExportData
	err := mapstructure.Decode(from, &exportData)
	return exportData, err
}
