package curseforge
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// AddonSlugRequest is sent to the CurseProxy GraphQL api to get the id from a slug
type AddonSlugRequest struct {
	Query     string `json:"query"`
	Variables struct {
		Slug string `json:"slug"`
	} `json:"variables"`
}

// AddonSlugResponse is received from the CurseProxy GraphQL api to get the id from a slug
type AddonSlugResponse struct {
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
	request := AddonSlugRequest{
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
	var response AddonSlugResponse
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

