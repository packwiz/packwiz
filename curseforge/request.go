package curseforge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// addonSlugRequest is sent to the CurseProxy GraphQL api to get the id from a slug
type addonSlugRequest struct {
	Query     string `json:"query"`
	Variables struct {
		Slug string `json:"slug"`
	} `json:"variables"`
}

// addonSlugResponse is received from the CurseProxy GraphQL api to get the id from a slug
type addonSlugResponse struct {
	Data struct {
		Addons []struct {
			ID int `json:"id"`
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
			{
				addons(slug: $slug) {
					id
				}
			}
		}
		`,
	}
	request.Variables.Slug = slug

	// Uses the curse.nikky.moe GraphQL api
	var response addonSlugResponse
	client := &http.Client{}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", "https://curse.nikky.moe/graphql", bytes.NewBuffer(requestBytes))
	if err != nil {
		return 0, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "comp500/packwiz client")
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
		return 0, fmt.Errorf("Error requesting id for slug: %s", response.Message)
	}

	if len(response.Data.Addons) < 1 {
		return 0, errors.New("Addon not found")
	}

	return response.Data.Addons[0].ID, nil
}

const (
	fileTypeRelease int = iota + 1
	fileTypeBeta
	fileTypeAlpha
)

const (
	dependencyTypeRequired int = iota + 1
	dependencyTypeOptional
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
		ID          int    `json:"projectFileId"`
		Name        string `json:"projectFileName"`
		FileType    int    `json:"fileType"`
	} `json:"gameVersionLatestFiles"`
}

func getModInfo(modID int) (modInfo, error) {
	var infoRes modInfo
	client := &http.Client{}

	idStr := strconv.Itoa(modID)

	req, err := http.NewRequest("GET", "https://addons-ecs.forgesvc.net/api/v2/addon/"+idStr, nil)
	if err != nil {
		return modInfo{}, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "comp500/packwiz client")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return modInfo{}, err
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return modInfo{}, err
	}

	if infoRes.ID != modID {
		return modInfo{}, fmt.Errorf("Unexpected addon ID in CurseForge response: %d/%d", modID, infoRes.ID)
	}

	return infoRes, nil
}

const cfDateFormatString = "2006-01-02T15:04:05.999"

type cfDateFormat struct {
	time.Time
}

func (f *cfDateFormat) UnmarshalJSON(input []byte) error {
	trimmed := strings.Trim(string(input), `"`)
	time, err := time.Parse(cfDateFormatString, trimmed)
	if err != nil {
		return err
	}

	f.Time = time
	return nil
}

// modFileInfo is a subset of the deserialised JSON response from the Curse API for mod files
type modFileInfo struct {
	ID           int          `json:"id"`
	FileName     string       `json:"fileNameOnDisk"`
	FriendlyName string       `json:"fileName"`
	Date         cfDateFormat `json:"fileDate"`
	Length       int          `json:"fileLength"`
	FileType     int          `json:"releaseType"`
	// fileStatus? means latest/preferred?
	DownloadURL  string   `json:"downloadUrl"`
	GameVersions []string `json:"gameVersion"`
	Fingerprint  int      `json:"packageFingerprint"`
	Dependencies []struct {
		ModID int `json:"addonId"`
		Type  int `json:"type"`
	} `json:"dependencies"`
}

func getFileInfo(modID int, fileID int) (modFileInfo, error) {
	var infoRes modFileInfo
	client := &http.Client{}

	modIDStr := strconv.Itoa(modID)
	fileIDStr := strconv.Itoa(fileID)

	req, err := http.NewRequest("GET", "https://addons-ecs.forgesvc.net/api/v2/addon/"+modIDStr+"/file/"+fileIDStr, nil)
	if err != nil {
		return modFileInfo{}, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "comp500/packwiz client")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return modFileInfo{}, err
	}

	err = json.NewDecoder(resp.Body).Decode(&infoRes)
	if err != nil && err != io.EOF {
		return modFileInfo{}, err
	}

	if infoRes.ID != fileID {
		return modFileInfo{}, fmt.Errorf("Unexpected file ID in CurseForge response: %d/%d", modID, infoRes.ID)
	}

	return infoRes, nil
}

