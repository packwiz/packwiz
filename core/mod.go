package core

import (
	"errors"
	"github.com/BurntSushi/toml"
	"io"
	"os"
	"path/filepath"
)

// Mod stores metadata about a mod. This is written to a TOML file for each mod.
type Mod struct {
	metaFile string      // The file for the metadata file, used as an ID
	Name     string      `toml:"name"`
	FileName string      `toml:"filename"`
	Side     string      `toml:"side,omitempty"`
	Download ModDownload `toml:"download"`
	// Update is a map of map of stuff, so you can store arbitrary values on string keys to define updating
	Update     map[string]map[string]interface{} `toml:"update"`
	updateData map[string]interface{}

	Option *ModOption `toml:"option,omitempty"`
}

const (
	modeURL string = "url"
	modeCF  string = "metadata:curseforge"
)

// ModDownload specifies how to download the mod file
type ModDownload struct {
	URL        string `toml:"url,omitempty"`
	HashFormat string `toml:"hash-format"`
	Hash       string `toml:"hash"`
	// Mode defaults to modeURL (i.e. use URL)
	Mode string `toml:"mode"`
}

// ModOption specifies optional metadata for this mod file
type ModOption struct {
	Optional    bool   `toml:"optional"`
	Description string `toml:"description,omitempty"`
	Default     bool   `toml:"default,omitempty"`
}

// The three possible values of Side (the side that the mod is on) are "server", "client", and "both".
//noinspection GoUnusedConst
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

// SetMetaPath sets the file path of a metadata file
func (m *Mod) SetMetaPath(metaFile string) string {
	m.metaFile = metaFile
	return m.metaFile
}

// Write saves the mod file, returning a hash format and the value of the hash of the saved file
func (m Mod) Write() (string, string, error) {
	f, err := os.Create(m.metaFile)
	if err != nil {
		// Attempt to create the containing directory
		err2 := os.MkdirAll(filepath.Dir(m.metaFile), os.ModePerm)
		if err2 == nil {
			f, err = os.Create(m.metaFile)
		}
		if err != nil {
			return "sha256", "", err
		}
	}

	h, err := GetHashImpl("sha256")
	if err != nil {
		_ = f.Close()
		return "", "", err
	}
	w := io.MultiWriter(h, f)

	enc := toml.NewEncoder(w)
	// Disable indentation
	enc.Indent = ""
	err = enc.Encode(m)
	hashString := h.HashToString(h.Sum(nil))
	if err != nil {
		_ = f.Close()
		return "sha256", hashString, err
	}
	return "sha256", hashString, f.Close()
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

// GetDestFilePath returns the path of the destination file of the mod
func (m Mod) GetDestFilePath() string {
	return filepath.Join(filepath.Dir(m.metaFile), filepath.FromSlash(m.FileName))
}
