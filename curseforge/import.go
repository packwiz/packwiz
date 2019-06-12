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
	// TODO: implement
	var packMeta twitchPackMeta
	// TODO: is this relative to something?
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	err = json.NewDecoder(f).Decode(&packMeta)
	if err != nil {
		return err
	}

	// TODO: magic involving existing files

	for _, v := range packMeta.Mods {
		// TODO: progress bar?

		// TODO: batch requests?
		modInfo, err := getModInfo(v.ID)
		if err != nil {
			// TODO: Fail more quietly?
			return cli.NewExitError(err, 1)
		}
		fmt.Println(v)
		fmt.Println(modFileInfo(v.File))

		err = createModFile(flags, modInfo, modFileInfo(v.File))
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	return nil
}

