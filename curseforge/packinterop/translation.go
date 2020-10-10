package packinterop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/comp500/packwiz/core"
	"io"
	"io/ioutil"
	"os"
)

func ReadMetadata(s ImportPackSource) ImportPackMetadata {
	var packImport ImportPackMetadata
	metaFile := s.GetPackFile()
	rdr, err := metaFile.Open()
	if err != nil {
		fmt.Printf("Error reading file: %s\n", err)
		os.Exit(1)
	}

	// Read the whole file (as we are going to parse it multiple times)
	fileData, err := ioutil.ReadAll(rdr)
	if err != nil {
		fmt.Printf("Error reading file: %s\n", err)
		os.Exit(1)
	}

	// Determine what format the file is
	var jsonFile map[string]interface{}
	err = json.Unmarshal(fileData, &jsonFile)
	if err != nil {
		fmt.Printf("Error parsing JSON: %s\n", err)
		os.Exit(1)
	}

	isManifest := false
	if v, ok := jsonFile["manifestType"]; ok {
		isManifest = v.(string) == "minecraftModpack"
	}
	if isManifest {
		packMeta := cursePackMeta{importSrc: s}
		err = json.Unmarshal(fileData, &packMeta)
		if err != nil {
			fmt.Printf("Error parsing JSON: %s\n", err)
			os.Exit(1)
		}
		packImport = packMeta
	} else {
		// Replace FileNameOnDisk with fileNameOnDisk
		fileData = bytes.ReplaceAll(fileData, []byte("FileNameOnDisk"), []byte("fileNameOnDisk"))
		packMeta := twitchInstalledPackMeta{importSrc: s}
		err = json.Unmarshal(fileData, &packMeta)
		if err != nil {
			fmt.Printf("Error parsing JSON: %s\n", err)
			os.Exit(1)
		}
		packImport = packMeta
	}

	return packImport
}

// AddonFileReference is a struct to reference a single file on CurseForge
type AddonFileReference struct {
	ProjectID int
	FileID    int
	// OptionalDisabled is true if the file is optional and disabled (turned off in Twitch launcher)
	OptionalDisabled bool
}

func WriteManifestFromPack(pack core.Pack, fileRefs []AddonFileReference, out io.Writer) error {
	files := make([]struct {
		ProjectID int  `json:"projectID"`
		FileID    int  `json:"fileID"`
		Required  bool `json:"required"`
	}, len(fileRefs))
	for i, fr := range fileRefs {
		files[i] = struct {
			ProjectID int  `json:"projectID"`
			FileID    int  `json:"fileID"`
			Required  bool `json:"required"`
		}{ProjectID: fr.ProjectID, FileID: fr.FileID, Required: !fr.OptionalDisabled}
	}

	modLoaders := make([]modLoaderDef, 0, 1)
	forgeVersion, ok := pack.Versions["forge"]
	if ok {
		modLoaders = append(modLoaders, modLoaderDef{
			ID:      "forge-" + forgeVersion,
			Primary: true,
		})
	}

	manifest := cursePackMeta{
		Minecraft: struct {
			Version    string         `json:"version"`
			ModLoaders []modLoaderDef `json:"modLoaders"`
		}{
			Version:    pack.Versions["minecraft"],
			ModLoaders: modLoaders,
		},
		ManifestType:    "minecraftModpack",
		ManifestVersion: 1,
		NameInternal:    pack.Name,
		Version:         pack.Version,
		Author:          pack.Author,
		ProjectID:       pack.ProjectID,
		Files:           files,
		Overrides:       "overrides",
	}

	w := json.NewEncoder(out)
	w.SetIndent("", "  ") // Match CF export
	err := w.Encode(manifest)
	if err != nil {
		return err
	}
	return nil
}
