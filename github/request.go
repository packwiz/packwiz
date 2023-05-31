package github

import (
	"fmt"
	"net/http"
)

const ghApiServer = "api.github.com"

type ghApiClient struct {
	httpClient *http.Client
}

var ghDefaultClient = ghApiClient{&http.Client{}}

func (c *ghApiClient) makeGet(slug string) (*http.Response, error) {
	req, err := http.NewRequest("GET", "https://" + ghApiServer + "/repos/" + slug + "/releases", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response status: %v", resp.Status)
	}
	return resp, nil
}
