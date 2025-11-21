package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/viper"
)

// Pack stores the modpack metadata, usually in pack.toml
type Pack struct {
	Name        string `toml:"name"`
	Author      string `toml:"author,omitempty"`
	Version     string `toml:"version,omitempty"`
	Description string `toml:"description,omitempty"`
	PackFormat  string `toml:"pack-format"`
	Index       struct {
		// Path is stored in forward slash format relative to pack.toml
		File       string `toml:"file"`
		HashFormat string `toml:"hash-format"`
		Hash       string `toml:"hash,omitempty"`
	} `toml:"index"`
	Versions map[string]string                 `toml:"versions"`
	Export   map[string]map[string]interface{} `toml:"export"`
	Options  map[string]interface{}            `toml:"options"`
}

const CurrentPackFormat = "packwiz:1.1.0"

var PackFormatConstraintAccepted = mustParseConstraint("~1")
var PackFormatConstraintSuggestUpgrade = mustParseConstraint("~1.1")

func mustParseConstraint(s string) *semver.Constraints {
	c, err := semver.NewConstraint(s)
	if err != nil {
		panic(err)
	}
	return c
}

// LoadPack loads the modpack metadata to a Pack struct
func LoadPack() (Pack, error) {
	var modpack Pack
	if _, err := toml.DecodeFile(viper.GetString("pack-file"), &modpack); err != nil {
		return Pack{}, err
	}

	// Check pack-format
	if len(modpack.PackFormat) == 0 {
		fmt.Println("Modpack manifest has no pack-format field; assuming packwiz:1.1.0")
		modpack.PackFormat = "packwiz:1.1.0"
	}
	// Auto-migrate versions
	if modpack.PackFormat == "packwiz:1.0.0" {
		fmt.Println("Automatically migrating pack to packwiz:1.1.0 format...")
		modpack.PackFormat = "packwiz:1.1.0"
	}
	if !strings.HasPrefix(modpack.PackFormat, "packwiz:") {
		return Pack{}, errors.New("pack-format field does not indicate a valid packwiz pack")
	}
	ver, err := semver.StrictNewVersion(strings.TrimPrefix(modpack.PackFormat, "packwiz:"))
	if err != nil {
		return Pack{}, fmt.Errorf("pack-format field is not valid semver: %w", err)
	}
	if !PackFormatConstraintAccepted.Check(ver) {
		return Pack{}, errors.New("the modpack is incompatible with this version of packwiz; please update")
	}
	if !PackFormatConstraintSuggestUpgrade.Check(ver) {
		fmt.Println("Modpack has a newer feature number than is supported by this version of packwiz. Update to the latest version of packwiz for new features and bugfixes!")
	}
	// TODO: suggest migration if necessary (primarily for 2.0.0)

	// Read options into viper
	if modpack.Options != nil {
		err := viper.MergeConfigMap(modpack.Options)
		if err != nil {
			return Pack{}, err
		}
	}

	if len(modpack.Index.File) == 0 {
		modpack.Index.File = "index.toml"
	}
	return modpack, nil
}

// LoadIndex attempts to load the index file of this modpack
func (pack Pack) LoadIndex() (Index, error) {
	if filepath.IsAbs(pack.Index.File) {
		return LoadIndex(pack.Index.File)
	}
	fileNative := filepath.FromSlash(pack.Index.File)
	return LoadIndex(filepath.Join(filepath.Dir(viper.GetString("pack-file")), fileNative))
}

// UpdateIndexHash recalculates the hash of the index file of this modpack
func (pack *Pack) UpdateIndexHash() error {
	if viper.GetBool("no-internal-hashes") {
		pack.Index.HashFormat = "sha256"
		pack.Index.Hash = ""
		return nil
	}

	fileNative := filepath.FromSlash(pack.Index.File)
	indexFile := filepath.Join(filepath.Dir(viper.GetString("pack-file")), fileNative)

	f, err := os.Open(indexFile)
	if err != nil {
		return err
	}

	// Hash usage strategy (may change):
	// Just use SHA256, overwrite existing hash regardless of what it is
	// May update later to continue using the same hash that was already being used
	h, err := GetHashImpl("sha256")
	if err != nil {
		_ = f.Close()
		return err
	}
	if _, err := io.Copy(h, f); err != nil {
		_ = f.Close()
		return err
	}
	hashString := h.HashToString(h.Sum(nil))

	pack.Index.HashFormat = "sha256"
	pack.Index.Hash = hashString
	return f.Close()
}

// Write saves the pack file
func (pack Pack) Write() error {
	f, err := os.Create(viper.GetString("pack-file"))
	if err != nil {
		return err
	}

	enc := toml.NewEncoder(f)
	// Disable indentation
	enc.Indent = ""
	err = enc.Encode(pack)
	if err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// GetMCVersion gets the version of Minecraft this pack uses, if it has been correctly specified
func (pack Pack) GetMCVersion() (string, error) {
	mcVersion, ok := pack.Versions["minecraft"]
	if !ok {
		return "", errors.New("no minecraft version specified in modpack")
	}
	return mcVersion, nil
}

// GetSupportedMCVersions gets the versions of Minecraft this pack allows in downloaded mods, ordered by preference (highest = most desirable)
func (pack Pack) GetSupportedMCVersions() ([]string, error) {
	mcVersion, ok := pack.Versions["minecraft"]
	if !ok {
		return nil, errors.New("no minecraft version specified in modpack")
	}
	allVersions := append(append([]string(nil), viper.GetStringSlice("acceptable-game-versions")...), mcVersion)
	// Deduplicate values
	allVersionsDeduped := []string(nil)
	for i, v := range allVersions {
		// If another copy of this value exists past this point in the array, don't insert
		// (i.e. prefer a later copy over an earlier copy, so the main version is last)
		if !slices.Contains(allVersions[i+1:], v) {
			allVersionsDeduped = append(allVersionsDeduped, v)
		}
	}
	return allVersionsDeduped, nil
}

func (pack Pack) GetPackName() string {
	if pack.Name == "" {
		return "export"
	} else if pack.Version == "" {
		return pack.Name
	} else {
		return pack.Name + "-" + pack.Version
	}
}

func (pack Pack) GetCompatibleLoaders() (loaders []string) {
	if _, hasQuilt := pack.Versions["quilt"]; hasQuilt {
		loaders = append(loaders, "quilt")
		loaders = append(loaders, "fabric") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasFabric := pack.Versions["fabric"]; hasFabric {
		loaders = append(loaders, "fabric")
	}
	if _, hasNeoForge := pack.Versions["neoforge"]; hasNeoForge {
		loaders = append(loaders, "neoforge")
		loaders = append(loaders, "forge") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasForge := pack.Versions["forge"]; hasForge {
		loaders = append(loaders, "forge")
	}
	return
}

func (pack Pack) GetLoaders() (loaders []string) {
	if _, hasQuilt := pack.Versions["quilt"]; hasQuilt {
		loaders = append(loaders, "quilt")
	}
	if _, hasFabric := pack.Versions["fabric"]; hasFabric {
		loaders = append(loaders, "fabric")
	}
	if _, hasNeoForge := pack.Versions["neoforge"]; hasNeoForge {
		loaders = append(loaders, "neoforge")
	}
	if _, hasForge := pack.Versions["forge"]; hasForge {
		loaders = append(loaders, "forge")
	}
	return
}
