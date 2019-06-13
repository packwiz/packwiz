package curseforge

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/comp500/packwiz/core"
	"github.com/urfave/cli"
)

type twitchPackMeta struct {
	Name string `json:"name"`
	Path string `json:"installPath"`
	// TODO: javaArgsOverride?
	// TODO: allocatedMemory?
	MCVersion string `json:"gameVersion"`
	Modloader struct {
		Name string `json:"name"`
	} `json:"baseModLoader"`
	// TODO: modpackOverrides?
	Mods []struct {
		ID   int `json:"addonID"`
		File struct {
			// This is exactly the same as modFileInfo, but with capitalised
			// FileNameOnDisk.
			ID           int          `json:"id"`
			FileName     string       `json:"FileNameOnDisk"`
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
		} `json:"installedFile"`
	} `json:"installedAddons"`
}

func cmdImport(flags core.Flags, file string) error {
	var packMeta twitchPackMeta
	// TODO: is this relative to something?
	f, err := os.Open(file)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = json.NewDecoder(f).Decode(&packMeta)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	modIDs := make([]int, len(packMeta.Mods))
	for i, v := range packMeta.Mods {
		modIDs[i] = v.ID
	}

	fmt.Println("Querying Curse API...")

	modInfos, err := getModInfoMultiple(modIDs)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	modInfosMap := make(map[int]modInfo)
	for _, v := range modInfos {
		modInfosMap[v.ID] = v
	}

	// TODO: multithreading????
	for _, v := range packMeta.Mods {
		modInfoValue, ok := modInfosMap[v.ID]
		if !ok {
			if len(v.File.FriendlyName) > 0 {
				fmt.Printf("Failed to obtain mod information for \"%s\"\n", v.File.FriendlyName)
			} else {
				fmt.Printf("Failed to obtain mod information for \"%s\"\n", v.File.FileName)
			}
			continue
		}

		fmt.Printf("Imported \"%s\" successfully!\n", modInfoValue.Name)

		err = createModFile(flags, modInfoValue, modFileInfo(v.File))
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	// TODO: import existing files (config etc.)

	return nil
}
