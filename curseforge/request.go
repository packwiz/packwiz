package curseforge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// TODO: update everything for no URL and download mode "metadata:curseforge"

const cfApiServer = "api.curseforge.com"

// If you fork/derive from packwiz, I request that you obtain your own API key.
const cfApiKeyDefault = "JDJhJDEwJHNBWVhqblU1N0EzSmpzcmJYM3JVdk92UWk2NHBLS3BnQ2VpbGc1TUM1UGNKL0RYTmlGWWxh"

// Exists so you can provide it as a build parameter: -ldflags="-X 'github.com/packwiz/packwiz/curseforge.cfApiKey=key'"
var cfApiKey = ""

func decodeDefaultKey() string {
	k, err := base64.StdEncoding.DecodeString(cfApiKeyDefault)
	if err != nil {
		panic("failed to read API key!")
	}
	return string(k)
}

type cfApiClient struct {
	httpClient *http.Client
}

var cfDefaultClient = cfApiClient{&http.Client{}}

func (c *cfApiClient) makeGet(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", "https://"+cfApiServer+endpoint, nil)
	if err != nil {
		return nil, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "packwiz/packwiz client")
	req.Header.Set("Accept", "application/json")
	if cfApiKey == "" {
		cfApiKey = decodeDefaultKey()
	}
	req.Header.Set("X-API-Key", cfApiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response status: %v", resp.Status)
	}
	return resp, nil
}

func (c *cfApiClient) makePost(endpoint string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", "https://"+cfApiServer+endpoint, body)
	if err != nil {
		return nil, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "packwiz/packwiz client")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if cfApiKey == "" {
		cfApiKey = decodeDefaultKey()
	}
	req.Header.Set("X-API-Key", cfApiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response status: %v", resp.Status)
	}
	return resp, nil
}

//noinspection GoUnusedConst
const (
	fileTypeRelease int = iota + 1
	fileTypeBeta
	fileTypeAlpha
)

//noinspection GoUnusedConst
const (
	dependencyTypeEmbedded int = iota + 1
	dependencyTypeOptional
	dependencyTypeRequired
	dependencyTypeTool
	dependencyTypeIncompatible
	dependencyTypeInclude
)

//noinspection GoUnusedConst
const (
	// modloaderTypeAny should not be passed to the API - it does not work
	modloaderTypeAny int = iota
	modloaderTypeForge
	modloaderTypeCauldron
	modloaderTypeLiteloader
	modloaderTypeFabric
)

var modloaderNames = [...]string{
	"",
	"Forge",
	"Cauldron",
	"Liteloader",
	"Fabric",
}

//noinspection GoUnusedConst
const (
	hashAlgoSHA1 int = iota + 1
	hashAlgoMD5
)

// modInfo is a subset of the deserialised JSON response from the Curse API for mods (addons)
type modInfo struct {
	Name                   string        `json:"name"`
	Summary                string        `json:"summary"`
	Slug                   string        `json:"slug"`
	ID                     int           `json:"id"`
	GameID                 uint32        `json:"gameId"`
	PrimaryCategoryID      uint32        `json:"primaryCategoryId"`
	ClassID                uint32        `json:"classId"`
	LatestFiles            []modFileInfo `json:"latestFiles"`
	GameVersionLatestFiles []struct {
		// TODO: check how twitch launcher chooses which one to use, when you are on beta/alpha channel?!
		// or does it not have the concept of release channels?!
		GameVersion string `json:"gameVersion"`
		ID          int    `json:"fileId"`
		Name        string `json:"filename"`
		FileType    int    `json:"releaseType"`
		Modloader   int    `json:"modLoader"`
	} `json:"latestFilesIndexes"`
	ModLoaders []string `json:"modLoaders"`
}

func (c *cfApiClient) getModInfo(modID int) (modInfo, error) {
	var infoRes struct {
		Data modInfo `json:"data"`
	}

	idStr := strconv.Itoa(modID)
	resp, err := c.makeGet("/v1/mods/" + idStr)
	if err != nil {
		return modInfo{}, fmt.Errorf("failed to request addon data for ID %d: %w", modID, err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return modInfo{}, fmt.Errorf("failed to request addon data for ID %d: %w", modID, err)
	}

	if infoRes.Data.ID != modID {
		return modInfo{}, fmt.Errorf("unexpected addon ID in CurseForge response: %d (expected %d)", infoRes.Data.ID, modID)
	}

	return infoRes.Data, nil
}

func (c *cfApiClient) getModInfoMultiple(modIDs []int) ([]modInfo, error) {
	var infoRes struct {
		Data []modInfo `json:"data"`
	}

	modIDsData, err := json.Marshal(struct {
		ModIDs []int `json:"modIds"`
	}{
		ModIDs: modIDs,
	})
	if err != nil {
		return []modInfo{}, err
	}

	resp, err := c.makePost("/v1/mods", bytes.NewBuffer(modIDsData))
	if err != nil {
		return []modInfo{}, fmt.Errorf("failed to request addon data: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return []modInfo{}, fmt.Errorf("failed to request addon data: %w", err)
	}

	return infoRes.Data, nil
}

// modFileInfo is a subset of the deserialised JSON response from the Curse API for mod files
type modFileInfo struct {
	ID           int       `json:"id"`
	FileName     string    `json:"fileName"`
	FriendlyName string    `json:"displayName"`
	Date         time.Time `json:"fileDate"`
	Length       int       `json:"fileLength"`
	FileType     int       `json:"releaseType"`
	// fileStatus? means latest/preferred?
	// According to the CurseForge API T&Cs, this must not be saved or cached
	DownloadURL  string   `json:"downloadUrl"`
	GameVersions []string `json:"gameVersions"`
	Fingerprint  int      `json:"fileFingerprint"`
	Dependencies []struct {
		ModID int `json:"modId"`
		Type  int `json:"relationType"`
	} `json:"dependencies"`

	Hashes []struct {
		Value     string `json:"value"`
		Algorithm int    `json:"algo"`
	} `json:"hashes"`
}

func (i modFileInfo) getBestHash() (hash string, hashFormat string) {
	// TODO: check if the hash is invalid (e.g. 0)
	hash = strconv.Itoa(i.Fingerprint)
	hashFormat = "murmur2"
	hashPreferred := 0

	// Prefer SHA1, then MD5 if found:
	if i.Hashes != nil {
		for _, v := range i.Hashes {
			if v.Algorithm == hashAlgoMD5 && hashPreferred < 1 {
				hashPreferred = 1

				hash = v.Value
				hashFormat = "md5"
			} else if v.Algorithm == hashAlgoSHA1 && hashPreferred < 2 {
				hashPreferred = 2

				hash = v.Value
				hashFormat = "sha1"
			}
		}
	}

	return
}

func (c *cfApiClient) getFileInfo(modID int, fileID int) (modFileInfo, error) {
	var infoRes struct {
		Data modFileInfo `json:"data"`
	}

	modIDStr := strconv.Itoa(modID)
	fileIDStr := strconv.Itoa(fileID)

	resp, err := c.makeGet("/v1/mods/" + modIDStr + "/files/" + fileIDStr)
	if err != nil {
		return modFileInfo{}, fmt.Errorf("failed to request file data for addon ID %d, file ID %d: %w", modID, fileID, err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return modFileInfo{}, fmt.Errorf("failed to request file data for addon ID %d, file ID %d: %w", modID, fileID, err)
	}

	if infoRes.Data.ID != fileID {
		return modFileInfo{}, fmt.Errorf("unexpected file ID for addon %d in CurseForge response: %d (expected %d)", modID, infoRes.Data.ID, fileID)
	}

	return infoRes.Data, nil
}

func (c *cfApiClient) getFileInfoMultiple(fileIDs []int) ([]modFileInfo, error) {
	var infoRes struct {
		Data []modFileInfo `json:"data"`
	}

	fileIDsData, err := json.Marshal(struct {
		FileIDs []int `json:"fileIds"`
	}{
		FileIDs: fileIDs,
	})
	if err != nil {
		return []modFileInfo{}, err
	}

	resp, err := c.makePost("/v1/mods/files", bytes.NewBuffer(fileIDsData))
	if err != nil {
		return []modFileInfo{}, fmt.Errorf("failed to request file data: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return []modFileInfo{}, fmt.Errorf("failed to request file data: %w", err)
	}

	return infoRes.Data, nil
}

func (c *cfApiClient) getSearch(searchTerm string, slug string, gameID int, classID int, categoryID int, gameVersion string, modloaderType int) ([]modInfo, error) {
	var infoRes struct {
		Data []modInfo `json:"data"`
	}

	q := url.Values{}
	q.Set("gameId", strconv.Itoa(gameID))
	q.Set("pageSize", "10")
	if classID != 0 {
		q.Set("classId", strconv.Itoa(classID))
	}
	if slug != "" {
		q.Set("slug", slug)
	}
	// If classID and slug are provided, don't bother filtering by anything else (should be unique)
	if classID == 0 && slug == "" {
		if categoryID != 0 {
			q.Set("categoryId", strconv.Itoa(categoryID))
		}
		if searchTerm != "" {
			q.Set("searchFilter", searchTerm)
		}
		if gameVersion != "" {
			q.Set("gameVersion", gameVersion)
		}
		if modloaderType != modloaderTypeAny {
			q.Set("modLoaderType", strconv.Itoa(modloaderType))
		}
	}

	resp, err := c.makeGet("/v1/mods/search?" + q.Encode())
	if err != nil {
		return []modInfo{}, fmt.Errorf("failed to retrieve search results: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return []modInfo{}, fmt.Errorf("failed to parse search results: %w", err)
	}

	return infoRes.Data, nil
}

//noinspection GoUnusedConst
const (
	gameStatusDraft int = iota + 1
	gameStatusTest
	gameStatusPendingReview
	gameStatusRejected
	gameStatusApproved
	gameStatusLive
)

//noinspection GoUnusedConst
const (
	gameApiStatusPrivate int = iota + 1
	gameApiStatusPublic
)

type cfGame struct {
	ID        uint32 `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Status    int    `json:"status"`
	APIStatus int    `json:"apiStatus"`
}

func (c *cfApiClient) getGames() ([]cfGame, error) {
	var infoRes struct {
		Data []cfGame `json:"data"`
	}

	resp, err := c.makeGet("/v1/games")
	if err != nil {
		return []cfGame{}, fmt.Errorf("failed to retrieve game list: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return []cfGame{}, fmt.Errorf("failed to parse game list: %w", err)
	}

	return infoRes.Data, nil
}

type cfCategory struct {
	ID      int    `json:"id"`
	Slug    string `json:"slug"`
	IsClass bool   `json:"isClass"`
	ClassID int    `json:"classId"`
}

func (c *cfApiClient) getCategories(gameID int) ([]cfCategory, error) {
	var infoRes struct {
		Data []cfCategory `json:"data"`
	}

	resp, err := c.makeGet("/v1/categories?gameId=" + strconv.Itoa(gameID))
	if err != nil {
		return []cfCategory{}, fmt.Errorf("failed to retrieve category list for game %v: %w", gameID, err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return []cfCategory{}, fmt.Errorf("failed to parse category list for game %v: %w", gameID, err)
	}

	return infoRes.Data, nil
}

type addonFingerprintResponse struct {
	IsCacheBuilt bool `json:"isCacheBuilt"`
	ExactMatches []struct {
		ID          int           `json:"id"`
		File        modFileInfo   `json:"file"`
		LatestFiles []modFileInfo `json:"latestFiles"`
	} `json:"exactMatches"`
	ExactFingerprints        []int    `json:"exactFingerprints"`
	PartialMatches           []int    `json:"partialMatches"`
	PartialMatchFingerprints struct{} `json:"partialMatchFingerprints"`
	InstalledFingerprints    []int    `json:"installedFingerprints"`
	UnmatchedFingerprints    []int    `json:"unmatchedFingerprints"`
}

func (c *cfApiClient) getFingerprintInfo(hashes []int) (addonFingerprintResponse, error) {
	var infoRes struct {
		Data addonFingerprintResponse `json:"data"`
	}

	hashesData, err := json.Marshal(struct {
		Fingerprints []int `json:"fingerprints"`
	}{
		Fingerprints: hashes,
	})
	if err != nil {
		return addonFingerprintResponse{}, err
	}

	resp, err := c.makePost("/v1/fingerprints", bytes.NewBuffer(hashesData))
	if err != nil {
		return addonFingerprintResponse{}, fmt.Errorf("failed to retrieve fingerprint results: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return addonFingerprintResponse{}, err
	}

	return infoRes.Data, nil
}
