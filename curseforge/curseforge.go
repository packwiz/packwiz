package curseforge

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/packwiz/packwiz/cmd"
	"github.com/packwiz/packwiz/core"
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
	core.MetaDownloaders["curseforge"] = cfDownloader{}
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

	if year >= 22 && week >= 11 {
		return "1.19-Snapshot"
	} else if year == 21 && week >= 37 || year >= 22 {
		return "1.18-Snapshot"
	} else if year == 20 && week >= 45 || year == 21 && week <= 20 {
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

var urlRegexes = [...]*regexp.Regexp{
	regexp.MustCompile("^https?://(?P<game>minecraft)\\.curseforge\\.com/projects/(?P<slug>[^/]+)(?:/(?:files|download)/(?P<fileID>\\d+))?"),
	regexp.MustCompile("^https?://(?:www\\.)?curseforge\\.com/(?P<game>[^/]+)/(?P<category>[^/]+)/(?P<slug>[^/]+)(?:/(?:files|download)/(?P<fileID>\\d+))?"),
	regexp.MustCompile("^(?P<slug>[a-z][\\da-z\\-_]{0,127})$"),
}

func parseSlugOrUrl(url string) (game string, category string, slug string, fileID int, err error) {
	for _, r := range urlRegexes {
		matches := r.FindStringSubmatch(url)
		if matches != nil {
			if i := r.SubexpIndex("game"); i >= 0 {
				game = matches[i]
			}
			if i := r.SubexpIndex("category"); i >= 0 {
				category = matches[i]
			}
			if i := r.SubexpIndex("slug"); i >= 0 {
				slug = matches[i]
			}
			if i := r.SubexpIndex("fileID"); i >= 0 {
				if matches[i] != "" {
					fileID, err = strconv.Atoi(matches[i])
				}
			}
			return
		}
	}
	return
}

var defaultFolders = map[uint32]map[uint32]string{
	432: { // Minecraft
		5:  "plugins", // Bukkit Plugins
		12: "resourcepacks",
		6:  "mods",
		17: "saves",
	},
}

func getPathForFile(gameID uint32, classID uint32, categoryID uint32, slug string) string {
	metaFolder := viper.GetString("meta-folder")
	if metaFolder == "" {
		if m, ok := defaultFolders[gameID]; ok {
			if folder, ok := m[classID]; ok {
				return filepath.Join(viper.GetString("meta-folder-base"), folder, slug+core.MetaExtension)
			} else if folder, ok := m[categoryID]; ok {
				return filepath.Join(viper.GetString("meta-folder-base"), folder, slug+core.MetaExtension)
			}
		}
		metaFolder = "."
	}
	return filepath.Join(viper.GetString("meta-folder-base"), metaFolder, slug+core.MetaExtension)
}

func createModFile(modInfo modInfo, fileInfo modFileInfo, index *core.Index, optionalDisabled bool) error {
	updateMap := make(map[string]map[string]interface{})
	var err error

	updateMap["curseforge"], err = cfUpdateData{
		ProjectID: modInfo.ID,
		FileID:    fileInfo.ID,
	}.ToMap()
	if err != nil {
		return err
	}

	hash, hashFormat := fileInfo.getBestHash()

	var optional *core.ModOption
	if optionalDisabled {
		optional = &core.ModOption{
			Optional: true,
			Default:  false,
		}
	}

	modMeta := core.Mod{
		Name:     modInfo.Name,
		FileName: fileInfo.FileName,
		Side:     core.UniversalSide,
		Download: core.ModDownload{
			HashFormat: hashFormat,
			Hash:       hash,
		},
		Option: optional,
		Update: updateMap,
	}
	path := modMeta.SetMetaPath(getPathForFile(modInfo.GameID, modInfo.ClassID, modInfo.PrimaryCategoryID, modInfo.Slug))

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
	_, hasQuilt := dependencies["quilt"]
	_, hasForge := dependencies["forge"]
	if (hasFabric || hasQuilt) && hasForge {
		return modloaderTypeAny
	} else if hasFabric || hasQuilt { // Backwards-compatible; for now (could be configurable later)
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
		containsLoader := false
		for i, name := range modloaderNames {
			if slices.Contains(fileInfoData.GameVersions, name) {
				containsLoader = true
				if i == packLoaderType {
					return true
				}
			}
		}
		// If a file doesn't contain any loaders, it matches all!
		return !containsLoader
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
	ProjectID int `mapstructure:"project-id"`
	FileID    int `mapstructure:"file-id"`
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

	modInfosUnsorted, err := cfDefaultClient.getModInfoMultiple(modIDs)
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
			fileInfoData, err = cfDefaultClient.getFileInfo(modState.ID, modState.fileID)
			if err != nil {
				return err
			}
		}

		v.FileName = fileInfoData.FileName
		v.Name = modState.Name
		hash, hashFormat := fileInfoData.getBestHash()
		v.Download = core.ModDownload{
			HashFormat: hashFormat,
			Hash:       hash,
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

type cfDownloader struct{}

func (c cfDownloader) GetFilesMetadata(mods []*core.Mod) ([]core.MetaDownloaderData, error) {
	if len(mods) == 0 {
		return []core.MetaDownloaderData{}, nil
	}

	downloaderData := make([]core.MetaDownloaderData, len(mods))
	indexMap := make(map[int]int)
	projectMetadata := make([]cfUpdateData, len(mods))
	modIDs := make([]int, len(mods))
	for i, v := range mods {
		updateData, ok := v.GetParsedUpdateData("curseforge")
		if !ok {
			return nil, fmt.Errorf("failed to read CurseForge update metadata from %s", v.Name)
		}
		project := updateData.(cfUpdateData)
		indexMap[project.ProjectID] = i
		projectMetadata[i] = project
		modIDs[i] = project.ProjectID
	}

	modData, err := cfDefaultClient.getModInfoMultiple(modIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get CurseForge mod metadata: %w", err)
	}

	handleFileInfo := func(modID int, fileInfo modFileInfo) {
		// If metadata already exists (i.e. opted-out) update it with more metadata
		if meta, ok := downloaderData[indexMap[modID]].(*cfDownloadMetadata); ok {
			if meta.noDistribution {
				meta.websiteUrl = meta.websiteUrl + "/files/" + strconv.Itoa(fileInfo.ID)
				meta.fileName = fileInfo.FileName
			}
		}
		downloaderData[indexMap[modID]] = &cfDownloadMetadata{
			url: fileInfo.DownloadURL,
		}
	}

	fileIDsToLookup := make([]int, 0)
	for _, mod := range modData {
		if _, ok := indexMap[mod.ID]; !ok {
			return nil, fmt.Errorf("unknown mod ID in response: %v (for %v)", mod.ID, mod.Name)
		}
		if !mod.AllowModDistribution {
			downloaderData[indexMap[mod.ID]] = &cfDownloadMetadata{
				noDistribution: true, // Inverted so the default value is not this (probably doesn't matter)
				name:           mod.Name,
				websiteUrl:     mod.Links.WebsiteURL,
			}
		}

		fileID := projectMetadata[indexMap[mod.ID]].FileID
		fileInfoFound := false
		// First look in latest files
		for _, fileInfo := range mod.LatestFiles {
			if fileInfo.ID == fileID {
				fileInfoFound = true
				handleFileInfo(mod.ID, fileInfo)
				break
			}
		}

		if !fileInfoFound {
			fileIDsToLookup = append(fileIDsToLookup, fileID)
		}
	}

	if len(fileIDsToLookup) > 0 {
		fileData, err := cfDefaultClient.getFileInfoMultiple(fileIDsToLookup)
		if err != nil {
			return nil, fmt.Errorf("failed to get CurseForge file metadata: %w", err)
		}
		for _, fileInfo := range fileData {
			if _, ok := indexMap[fileInfo.ModID]; !ok {
				return nil, fmt.Errorf("unknown mod ID in response: %v from file %v (for %v)", fileInfo.ModID, fileInfo.ID, fileInfo.FileName)
			}
			handleFileInfo(fileInfo.ModID, fileInfo)
		}
	}

	return downloaderData, nil
}

type cfDownloadMetadata struct {
	url            string
	noDistribution bool
	name           string
	fileName       string
	websiteUrl     string
}

func (m *cfDownloadMetadata) GetManualDownload() (bool, core.ManualDownload) {
	if !m.noDistribution {
		return false, core.ManualDownload{}
	}
	return true, core.ManualDownload{
		Name:     m.name,
		FileName: m.fileName,
		URL:      m.websiteUrl,
	}
}

func (m *cfDownloadMetadata) DownloadFile() (io.ReadCloser, error) {
	resp, err := http.Get(m.url)
	// TODO: content type, user-agent?
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", m.url, err)
	}
	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("failed to download %s: invalid status code %v", m.url, resp.StatusCode)
	}
	return resp.Body, nil
}
