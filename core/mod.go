package core
import "github.com/BurntSushi/toml"

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
	Update map[string]toml.Primitive `toml:"update"`
}

// The three possible values of Side (the side that the mod is on) are "server", "client", and "both".
const (
	ServerSide    = "server"
	ClientSide    = "client"
	UniversalSide = "both"
)

func (m Mod) Write() {

}

