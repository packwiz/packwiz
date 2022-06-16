package packinterop

import (
	"path/filepath"
	"strings"
)

type twitchInstalledPackMeta struct {
	NameInternal string `json:"name"`
	Path         string `json:"installPath"`
	// TODO: javaArgsOverride?
	// TODO: allocatedMemory?
	MCVersion string `json:"gameVersion"`
	Modloader struct {
		Name               string `json:"name"`
		MavenVersionString string `json:"mavenVersionString"`
	} `json:"baseModLoader"`
	ModpackOverrides []string `json:"modpackOverrides"`
	ModsInternal     []struct {
		ID   uint32 `json:"addonID"`
		File struct {
			// I've given up on using this cached data, just going to re-request it
			ID uint32 `json:"id"`
			// Used to determine if the mod is optional-disabled
			FileNameOnDisk string
		} `json:"installedFile"`
	} `json:"installedAddons"`
	// Used to determine if modpackOverrides should be used or not
	IsUnlocked bool `json:"isUnlocked"`
	importSrc  ImportPackSource
}

func (m twitchInstalledPackMeta) Name() string {
	return m.NameInternal
}

func (m twitchInstalledPackMeta) PackAuthor() string {
	return ""
}

func (m twitchInstalledPackMeta) PackVersion() string {
	return ""
}

func (m twitchInstalledPackMeta) Versions() map[string]string {
	vers := make(map[string]string)
	vers["minecraft"] = m.MCVersion
	if strings.HasPrefix(m.Modloader.Name, "forge") {
		if len(m.Modloader.MavenVersionString) > 0 {
			vers["forge"] = strings.TrimPrefix(m.Modloader.MavenVersionString, "net.minecraftforge:forge:")
		} else {
			vers["forge"] = strings.TrimPrefix(m.Modloader.Name, "forge-")
		}
		// Remove the minecraft version prefix, if it exists
		vers["forge"] = strings.TrimPrefix(vers["forge"], m.MCVersion+"-")
	} else if strings.HasPrefix(m.Modloader.Name, "fabric") {
		if len(m.Modloader.MavenVersionString) > 0 {
			vers["fabric"] = strings.TrimPrefix(m.Modloader.MavenVersionString, "net.fabricmc:fabric-loader:")
		} else {
			vers["fabric"] = strings.TrimPrefix(m.Modloader.Name, "fabric-")
		}
		// Remove the minecraft version suffix, if it exists
		vers["fabric"] = strings.TrimSuffix(vers["fabric"], m.MCVersion+"-")
	}
	return vers
}

func (m twitchInstalledPackMeta) Mods() []AddonFileReference {
	list := make([]AddonFileReference, len(m.ModsInternal))
	for i, v := range m.ModsInternal {
		list[i] = AddonFileReference{
			ProjectID:        v.ID,
			FileID:           v.File.ID,
			OptionalDisabled: strings.HasSuffix(v.File.FileNameOnDisk, ".disabled"),
		}
	}
	return list
}

func (m twitchInstalledPackMeta) GetFiles() ([]ImportPackFile, error) {
	// If the modpack is unlocked, import all the files
	// Otherwise import just the modpack overrides
	if m.IsUnlocked {
		return m.importSrc.GetFileList()
	}
	list := make([]ImportPackFile, len(m.ModpackOverrides))
	var err error
	for i, v := range m.ModpackOverrides {
		list[i], err = m.importSrc.GetFile(filepath.ToSlash(v))
		if err != nil {
			return nil, err
		}
	}
	return list, nil
}
