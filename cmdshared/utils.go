package cmdshared

import "strings"

func GetRawForgeVersion(version string) string {
	var wantedVersion string
	// Check if we have a "-" in the version
	if strings.Contains(version, "-") {
		// We have a mcVersion-loaderVersion format
		// Strip the mcVersion
		wantedVersion = strings.Split(version, "-")[1]
	} else {
		wantedVersion = version
	}
	return wantedVersion
}
