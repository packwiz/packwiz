package core

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
)

// Pack stores the modpack metadata, usually in pack.toml
type Pack struct {
	Name  string `toml:"name"`
	Index struct {
		// Path is stored in forward slash format relative to pack.toml
		File       string `toml:"file"`
		HashFormat string `toml:"hash-format"`
		Hash       string `toml:"hash"`
	} `toml:"index"`
	Versions map[string]string         `toml:"versions"`
	Client   map[string]toml.Primitive `toml:"client"`
	Server   map[string]toml.Primitive `toml:"server"`
}

// LoadPack loads the modpack metadata to a Pack struct
func LoadPack() (Pack, error) {
	var modpack Pack
	if _, err := toml.DecodeFile(viper.GetString("pack-file"), &modpack); err != nil {
		return Pack{}, err
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
	fileNative := filepath.FromSlash(pack.Index.File)
	indexFile := filepath.Join(filepath.Dir(viper.GetString("pack-file")), fileNative)
	if filepath.IsAbs(pack.Index.File) {
		indexFile = pack.Index.File
	}

	f, err := os.Open(indexFile)
	if err != nil {
		return err
	}
	defer f.Close()

	// Hash usage strategy (may change):
	// Just use SHA256, overwrite existing hash regardless of what it is
	// May update later to continue using the same hash that was already being used
	h, err := GetHashImpl("sha256")
	if err != nil {
		return err
	}
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	hashString := hex.EncodeToString(h.Sum(nil))

	pack.Index.HashFormat = "sha256"
	pack.Index.Hash = hashString
	return nil
}

// Write saves the pack file
func (pack Pack) Write() error {
	f, err := os.Create(viper.GetString("pack-file"))
	if err != nil {
		return err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	// Disable indentation
	enc.Indent = ""
	return enc.Encode(pack)
}

// GetMCVersion gets the version of Minecraft this pack uses, if it has been correctly specified
func (pack Pack) GetMCVersion() (string, error) {
	mcVersion, ok := pack.Versions["minecraft"]
	if !ok {
		return "", errors.New("no minecraft version specified in modpack")
	}
	return mcVersion, nil
}
