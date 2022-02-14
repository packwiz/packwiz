package core

import (
	"fmt"
	"net/url"
)

func ReencodeURL(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("failed to parse url: %s, %v", u, err)
	}
	return parsed.String(), nil
}
