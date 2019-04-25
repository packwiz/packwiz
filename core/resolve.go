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

// ResolveIndex returns the path to the index file
func ResolveIndex(flags Flags) (string, error) {
	pack, err := LoadPack(flags)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(pack.Index.File) {
		return pack.Index.File, nil
	}
	return filepath.Join(flags.PackFile, pack.Index.File), nil
}

