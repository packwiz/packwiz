package packinterop

import "strings"

type cursePackMeta struct {
	Minecraft struct {
		Version    string         `json:"version"`
		ModLoaders []modLoaderDef `json:"modLoaders"`
	} `json:"minecraft"`
	ManifestType    string `json:"manifestType"`
	ManifestVersion uint32 `json:"manifestVersion"`
	NameInternal    string `json:"name"`
	Version         string `json:"version"`
	Author          string `json:"author"`
	ProjectID       uint32 `json:"projectID"`
	Files           []struct {
		ProjectID uint32 `json:"projectID"`
		FileID    uint32 `json:"fileID"`
		Required  bool   `json:"required"`
	} `json:"files"`
	Overrides string `json:"overrides"`
	importSrc ImportPackSource
}

type modLoaderDef struct {
	ID      string `json:"id"`
	Primary bool   `json:"primary"`
}

func (c cursePackMeta) Name() string {
	return c.NameInternal
}

func (c cursePackMeta) PackVersion() string {
	return c.Version
}

func (c cursePackMeta) PackAuthor() string {
	return c.Author
}

func (c cursePackMeta) Versions() map[string]string {
	vers := make(map[string]string)
	vers["minecraft"] = c.Minecraft.Version
	for _, v := range c.Minecraft.ModLoaders {
		// Seperate dash-separated modloader/version pairs
		parts := strings.SplitN(v.ID, "-", 2)
		if len(parts) == 2 {
			vers[parts[0]] = parts[1]
		}
	}
	if val, ok := vers["forge"]; ok {
		// Remove the minecraft version prefix, if it exists
		vers["forge"] = strings.TrimPrefix(val, c.Minecraft.Version+"-")
	}
	return vers
}

func (c cursePackMeta) Mods() []AddonFileReference {
	list := make([]AddonFileReference, len(c.Files))
	for i, v := range c.Files {
		list[i] = AddonFileReference{
			ProjectID:        v.ProjectID,
			FileID:           v.FileID,
			OptionalDisabled: !v.Required,
		}
	}
	return list
}

type cursePackOverrideWrapper struct {
	name string
	ImportPackFile
}

func (w cursePackOverrideWrapper) Name() string {
	return w.name
}

func (c cursePackMeta) GetFiles() ([]ImportPackFile, error) {
	// Only import files from overrides directory
	if len(c.Overrides) > 0 {
		fullList, err := c.importSrc.GetFileList()
		if err != nil {
			return nil, err
		}
		overridesList := make([]ImportPackFile, 0, len(fullList))
		overridesPath := c.Overrides
		if !strings.HasSuffix(overridesPath, "/") {
			overridesPath = c.Overrides + "/"
		}
		// Wrap files, removing overrides/ from the start
		for _, v := range fullList {
			if strings.HasPrefix(v.Name(), overridesPath) {
				overridesList = append(overridesList, cursePackOverrideWrapper{
					name:           strings.TrimPrefix(v.Name(), overridesPath),
					ImportPackFile: v,
				})
			}
		}
		return overridesList, nil
	}
	return []ImportPackFile{}, nil
}
