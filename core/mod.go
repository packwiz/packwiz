package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

// Mod stores metadata about a mod. This is written to a TOML file for each mod.
type Mod struct {
	metaFile string      // The file for the metadata file, used as an ID
	Name     string      `toml:"name"`
	FileName string      `toml:"filename"`
	Side     string      `toml:"side,omitempty"`
	Optional bool        `toml:"optional,omitempty"`
	Download ModDownload `toml:"download"`
	// Update is a map of map of stuff, so you can store arbitrary values on string keys to define updating
	Update     map[string]map[string]interface{} `toml:"update"`
	updateData map[string]interface{}
}

// ModDownload specifies how to download the mod file
type ModDownload struct {
	URL        string `toml:"url"`
	HashFormat string `toml:"hash-format"`
	Hash       string `toml:"hash"`
}

// The three possible values of Side (the side that the mod is on) are "server", "client", and "both".
const (
	ServerSide    = "server"
	ClientSide    = "client"
	UniversalSide = "both"
)

// LoadMod attempts to load a mod file from a path
func LoadMod(modFile string) (Mod, error) {
	var mod Mod
	if _, err := toml.DecodeFile(modFile, &mod); err != nil {
		return Mod{}, err
	}
	mod.updateData = make(map[string]interface{})
	// Horrible reflection library to convert map[string]interface to proper struct
	for k, v := range mod.Update {
		updater, ok := Updaters[k]
		if ok {
			updateData, err := updater.ParseUpdate(v)
			if err != nil {
				return mod, err
			}
			mod.updateData[k] = updateData
		} else {
			return mod, errors.New("Update plugin " + k + " not found!")
		}
	}
	mod.metaFile = modFile
	return mod, nil
}

// SetMetaName sets the mod metadata file from a given file name (to be put in the mods folder)
func (m *Mod) SetMetaName(metaName string, flags Flags) string {
	m.metaFile = ResolveMod(metaName, flags)
	return m.metaFile
}

// Write saves the mod file, returning a hash format and the value of the hash of the saved file
func (m Mod) Write() (string, string, error) {
	f, err := os.Create(m.metaFile)
	if err != nil {
		return "sha256", "", err
	}
	defer f.Close()

	h := sha256.New()
	w := io.MultiWriter(h, f)

	enc := toml.NewEncoder(w)
	// Disable indentation
	enc.Indent = ""
	err = enc.Encode(m)
	hashString := hex.EncodeToString(h.Sum(nil))
	return "sha256", hashString, err
}

// GetParsedUpdateData can be used to retrieve updater-specific information after parsing a mod file
func (m Mod) GetParsedUpdateData(updaterName string) (interface{}, bool) {
	upd, ok := m.updateData[updaterName]
	return upd, ok
}

// GetFilePath is a clumsy hack that I made because Mod already stores it's path anyway
func (m Mod) GetFilePath() string {
	return m.metaFile
}