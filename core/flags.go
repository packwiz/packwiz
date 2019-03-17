package core
import (
	"github.com/urfave/cli"
)

// Flags stores common information passed as flags to the program.
type Flags struct {
	PackFile   string
	ModsFolder string
}

// FlagsFromContext converts a CLI context (from commands) into a Flags struct, for use in helper functions.
func FlagsFromContext(c *cli.Context) Flags {
	return Flags{
		c.GlobalString("pack-file"),
		c.GlobalString("mods-folder"),
	}
}

// CLIFlags is used internally to initialise the internal flags (easier to keep in one place)
var CLIFlags = [...]cli.Flag{
	cli.StringFlag{
		Name:  "pack-file",
		Value: "pack.toml",
		Usage: "The modpack metadata file to use",
	},
	cli.StringFlag{
		Name:  "mods-folder",
		Value: "mods",
		Usage: "The mods folder to use",
	},
}

