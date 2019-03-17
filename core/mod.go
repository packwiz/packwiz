package core

import "github.com/BurntSushi/toml"

// Mod stores metadata about a mod. This is written to a TOML file for each mod.
type Mod struct {
	metaFilename string
	Name     string `toml:"name"`
	FileName string `toml:"filename"`
	Side     string `toml:"side"`
	Optional bool   `toml:"optional"`
	Download struct {
		URL        string `toml:"url"`
		HashFormat string `toml:"hash-format"`
		Hash       string `toml:"hash"`
	} `toml:"download"`
	Update map[string]toml.Primitive
}
// The three possible values of Side (the side that the mod is on) are "server", "client", and "both".
const (
	ServerSide    = "server"
	ClientSide    = "client"
	UniversalSide = "both"
)

func (m Mod) Write() {

}

