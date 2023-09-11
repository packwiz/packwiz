package github

import (
	"fmt"
	"net/http"

	"github.com/packwiz/packwiz/core"
	"github.com/spf13/viper"
)

// TODO: allow setting github api key via env variable
const ghApiServer = "api.github.com"

type ghApiClient struct {
	httpClient *http.Client
}

var ghDefaultClient = ghApiClient{&http.Client{}}

func (c *ghApiClient) makeGet(url string) (*http.Response, error) {
	ghApiToken := viper.GetString("github.token")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", core.UserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	if ghApiToken != "" {
		req.Header.Set("Authorization", "Bearer " + ghApiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response status: %v", resp.Status)
	}

	return resp, nil
}

func (c *ghApiClient) getRepo(slug string) (*http.Response, error) {
	resp, err := c.makeGet("https://" + ghApiServer + "/repos/" + slug)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

func (c *ghApiClient) getReleases(slug string) (*http.Response, error) {
	resp, err := c.getRepo(slug + "/releases")
	if err != nil {
		return resp, err
	}

	return resp, nil
}
