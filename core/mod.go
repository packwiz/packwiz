package core

import (
	"errors"
	"fmt"
	"golang.org/x/exp/slices"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
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

// ModDownload specifies how to download the mod file
type ModDownload struct {
	URL        string `toml:"url"`
	HashFormat string `toml:"hash-format"`
	Hash       string `toml:"hash"`
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

// SetMetaName sets the mod metadata file from a given file name (to be put in the mods folder)
func (m *Mod) SetMetaName(metaName string, index Index) string {
	m.metaFile = ResolveMod(metaName, index)
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

// DownloadFile attempts to resolve and download the file
func (m Mod) DownloadFile(dest io.Writer) error {
	resp, err := http.Get(m.Download.URL)
	// TODO: content type, user-agent?
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return errors.New("invalid status code " + strconv.Itoa(resp.StatusCode))
	}
	h, err := GetHashImpl(m.Download.HashFormat)
	if err != nil {
		return fmt.Errorf("failed to get hash format %s to download file: %w", m.Download.HashFormat, err)
	}

	w := io.MultiWriter(h, dest)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}

	calculatedHash := h.HashToString(h.Sum(nil))

	// Check if the hash of the downloaded file matches the expected hash.
	if calculatedHash != m.Download.Hash {
		return fmt.Errorf("Hash of downloaded file does not match with expected hash!\n download hash: %s\n expected hash: %s\n", calculatedHash, m.Download.Hash)
	}

	return nil
}

// GetHashes attempts to retrieve the values of all hashes passed to it, downloading if necessary
func (m Mod) GetHashes(hashes []string) (map[string]string, error) {
	out := make(map[string]string)

	// Get the hash already stored TODO: store multiple (requires breaking pack change)
	if m.Download.Hash != "" {
		idx := slices.Index(hashes, m.Download.HashFormat)
		if idx > -1 {
			out[m.Download.HashFormat] = m.Download.Hash
			// Remove hash from list to retrieve
			hashes = slices.Delete(hashes, idx, idx+1)
		}
	}

	// Retrieve the remaining hashes
	if len(hashes) > 0 {
		resp, err := http.Get(m.Download.URL)
		// TODO: content type, user-agent?
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			_ = resp.Body.Close()
			return nil, errors.New("invalid status code " + strconv.Itoa(resp.StatusCode))
		}

		// Special fast-path for file length only
		if len(hashes) == 1 && hashes[0] == "length-bytes" && resp.ContentLength > 0 {
			out["length-bytes"] = strconv.FormatInt(resp.ContentLength, 10)
			_ = resp.Body.Close()
			return out, nil
		}

		mainHasher, err := GetHashImpl(m.Download.HashFormat)
		if err != nil {
			return nil, fmt.Errorf("failed to get hash format %s to download file: %w", m.Download.HashFormat, err)
		}

		hashers := make([]HashStringer, len(hashes))
		allHashers := make([]io.Writer, len(hashers))
		for i, v := range hashes {
			hashers[i], err = GetHashImpl(v)
			if err != nil {
				return nil, fmt.Errorf("failed to get hash format %s for file: %w", v, err)
			}
			allHashers[i] = hashers[i]
		}
		allHashers = append(allHashers, mainHasher)

		w := io.MultiWriter(allHashers...)
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to download file: %w", err)
		}

		calculatedHash := mainHasher.HashToString(mainHasher.Sum(nil))

		// Check if the hash of the downloaded file matches the expected hash
		if calculatedHash != m.Download.Hash {
			return nil, fmt.Errorf("Hash of downloaded file does not match with expected hash!\n download hash: %s\n expected hash: %s\n", calculatedHash, m.Download.Hash)
		}

		for i, v := range hashers {
			out[hashes[i]] = v.HashToString(v.Sum(nil))
		}
	}
	return out, nil
}
