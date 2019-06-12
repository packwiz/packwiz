package curseforge
import "github.com/comp500/packwiz/core"

type twitchPackMeta struct {
	Name string `json:"name"`
	Path string `json:"installPath"`
	// TODO: javaArgsOverride?
	// TODO: allocatedMemory?
	MCVersion string `json:"gameVersion"`
	Modloader struct {
		Name string `json:"name"`
	} `json:"baseModLoader"`
	// TODO: modpackOverrides?
	Mods []struct {
		ID   int `json:"addonID"`
		File struct {
			// TODO: this is exactly the same as fileInfo, but with capitalised
			// FileNameOnDisk. Move requesting stuff out of createModFile?
		} `json:"installedFile"`
	} `json:"installedAddons"`
}

func cmdImport(flags core.Flags, file string) error {
	// TODO: implement
	return nil
}

