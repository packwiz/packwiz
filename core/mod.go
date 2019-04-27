package core
import (
	"errors"

	"github.com/BurntSushi/toml"
)

// Mod stores metadata about a mod. This is written to a TOML file for each mod.
type Mod struct {
	metaFilename string // The filename for the metadata file, used as an ID
	Name         string `toml:"name"`
	FileName     string `toml:"filename"`
	Side         string `toml:"side,omitempty"`
	Optional     bool   `toml:"optional,omitempty"`
	Download     struct {
		URL        string `toml:"url"`
		HashFormat string `toml:"hash-format"`
		Hash       string `toml:"hash"`
	} `toml:"download"`
	Update map[string]interface{} `toml:"update"`
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
	// Horrible reflection library to convert to Updaters
	for k, v := range mod.Update {
		updateParser, ok := UpdateParsers[k]
		if ok {
			updater, err := updateParser.ParseUpdate(v)
			if err != nil {
				return mod, err
			}
			mod.Update[k] = updater
		} else {
			return mod, errors.New("Update plugin " + k + " not found!")
		}
	}
	return mod, nil
}

func (m Mod) Write() {

}

