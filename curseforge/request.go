package curseforge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// addonSlugRequest is sent to the CurseProxy GraphQL api to get the id from a slug
type addonSlugRequest struct {
	Query     string `json:"query"`
	Variables struct {
		Slug string `json:"slug"`
	} `json:"variables"`
	OperationName string `json:"operationName"`
}

// addonSlugResponse is received from the CurseProxy GraphQL api to get the id from a slug
type addonSlugResponse struct {
	Data struct {
		Addons []struct {
			ID              int `json:"id"`
			CategorySection struct {
				ID int `json:"id"`
			} `json:"categorySection"`
		} `json:"addons"`
	} `json:"data"`
	Exception  string   `json:"exception"`
	Message    string   `json:"message"`
	Stacktrace []string `json:"stacktrace"`
}

// Most of this is shamelessly copied from my previous attempt at modpack management:
// https://github.com/comp500/modpack-editor/blob/master/query.go
func modIDFromSlug(slug string) (int, error) {
	request := addonSlugRequest{
		Query: `
		query getIDFromSlug($slug: String) {
			addons(slug: $slug) {
				id
				categorySection {
					id
				}
			}
		}
		`,
		OperationName: "getIDFromSlug",
	}
	request.Variables.Slug = slug

	// Uses the curse.nikky.moe GraphQL api
	var response addonSlugResponse
	client := &http.Client{}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	// TODO: move to new slug API
	req, err := http.NewRequest("POST", "https://curse.nikky.moe/graphql", bytes.NewBuffer(requestBytes))
	if err != nil {
		return 0, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "packwiz/packwiz client")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		return 0, err
	}

	if len(response.Exception) > 0 || len(response.Message) > 0 {
		return 0, fmt.Errorf("error requesting id for slug: %s", response.Message)
	}

	for _, addonData := range response.Data.Addons {
		// Only use mods, not resource packs/modpacks
		if addonData.CategorySection.ID == 8 {
			return addonData.ID, nil
		}
	}
	return 0, errors.New("addon not found")
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

//noinspection GoUnusedConst
const (
	hashAlgoSHA1 int = iota + 1
	hashAlgoMD5
)

// modInfo is a subset of the deserialised JSON response from the Curse API for mods (addons)
type modInfo struct {
	Name                   string        `json:"name"`
	Slug                   string        `json:"slug"`
	ID                     int           `json:"id"`
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

func (c *cfApiClient) getSearch(searchText string, gameVersion string, modloaderType int) ([]modInfo, error) {
	var infoRes struct {
		Data []modInfo `json:"data"`
	}

	q := url.Values{}
	q.Set("gameId", "432") // Minecraft
	q.Set("pageSize", "10")
	q.Set("classId", "6") // Mods
	q.Set("searchFilter", searchText)
	if len(gameVersion) > 0 {
		q.Set("gameVersion", gameVersion)
	}
	if modloaderType != modloaderTypeAny {
		q.Set("modLoaderType", strconv.Itoa(modloaderType))
	}

	resp, err := c.makeGet("/v1/mods/search?" + q.Encode())
	if err != nil {
		return []modInfo{}, fmt.Errorf("failed to retrieve search results: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return []modInfo{}, err
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
