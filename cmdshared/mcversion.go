package cmdshared

import (
	"encoding/json"
	"fmt"
	"packwiz/core"
	"os"
	"sort"
	"time"
)

type McVersionManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []struct {
		ID          string    `json:"id"`
		Type        string    `json:"type"`
		URL         string    `json:"url"`
		Time        time.Time `json:"time"`
		ReleaseTime time.Time `json:"releaseTime"`
	} `json:"versions"`
}

func (m McVersionManifest) CheckValid(version string) {
	for _, v := range m.Versions {
		if v.ID == version {
			return
		}
	}
	fmt.Println("Given version is not a valid Minecraft version!")
	os.Exit(1)
}

func GetValidMCVersions() (McVersionManifest, error) {
	res, err := core.GetWithUA("https://launchermeta.mojang.com/mc/game/version_manifest.json", "application/json")
	if err != nil {
		return McVersionManifest{}, err
	}
	dec := json.NewDecoder(res.Body)
	out := McVersionManifest{}
	err = dec.Decode(&out)
	if err != nil {
		return McVersionManifest{}, err
	}
	// Sort by newest to oldest
	sort.Slice(out.Versions, func(i, j int) bool {
		return out.Versions[i].ReleaseTime.Before(out.Versions[j].ReleaseTime)
	})
	return out, nil
}
