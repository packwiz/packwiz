package core
import (
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

// Pack stores the modpack metadata, usually in pack.toml
type Pack struct {
	Name  string `toml:"name"`
	Index struct {
		File       string `toml:"file"`
		HashFormat string `toml:"hash-format"`
		Hash       string `toml:"hash"`
	} `toml:"index"`
	Versions map[string]string         `toml:"versions"`
	Client   map[string]toml.Primitive `toml:"client"`
	Server   map[string]toml.Primitive `toml:"server"`
}

// LoadPack loads the modpack metadata to a Pack struct
func LoadPack(flags Flags) (Pack, error) {
	data, err := ioutil.ReadFile(flags.PackFile)
	if err != nil {
		return Pack{}, err
	}
	var modpack Pack
	if _, err := toml.Decode(string(data), &modpack); err != nil {
		return Pack{}, err
	}

	if len(modpack.Index.File) == 0 {
		modpack.Index.File = "index.toml"
	}
	return modpack, nil
}

