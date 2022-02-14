package core

import (
	"fmt"
	"net/url"
	"strings"
)

// ReencodeURL re-encodes URLs for RFC3986 compliance; as CurseForge URLs aren't properly encoded
func ReencodeURL(u string) (string, error) {
	// Go's URL library isn't entirely RFC3986 compliant :(
	// Manually replace [ and ] with %5B and %5D
	u = strings.ReplaceAll(u, "[", "%5B")
	u = strings.ReplaceAll(u, "]", "%5D")
	parsed, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("failed to parse url: %s, %v", u, err)
	}
	return parsed.String(), nil
}
