package core
import (
	"path/filepath"
	"strings"
)

// ModExtension is the file extension of the mod metadata files
const ModExtension = ".toml"

// ResolveMod returns the path to a mod file from it's name
func ResolveMod(modName string, flags Flags) string {
	fileName := strings.ToLower(strings.TrimSuffix(modName, ModExtension)) + ModExtension
	return filepath.Join(flags.ModsFolder, fileName)
}

