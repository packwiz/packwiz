package curseforge

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func getCurseDir() (string, error) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil {
		return "", err
	}
	curseDir := filepath.Join(path, "Curse")
	if _, err := os.Stat(curseDir); err == nil {
		return curseDir, nil
	}
	curseDir = filepath.Join(path, "Twitch")
	if _, err := os.Stat(curseDir); err == nil {
		return curseDir, nil
	}
	return "", errors.New("curse installation directory cannot be found")
}
