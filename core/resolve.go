package core

import (
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// ModExtension is the file extension of the mod metadata files
const ModExtension = ".toml"

// ResolveMod returns the path to a mod file from it's name
func ResolveMod(modName string) string {
	// TODO: should this work for any metadata file?
	fileName := strings.ToLower(strings.TrimSuffix(modName, ModExtension)) + ModExtension
	return filepath.Join(viper.GetString("mods-folder"), fileName)
}
