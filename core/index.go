package core

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Index is a representation of the index.toml file for referencing all the files in a pack.
type Index struct {
	HashFormat string      `toml:"hash-format"`
	Files      []IndexFile `toml:"files"`
	flags      Flags
	indexFile  string
}

// IndexFile is a file in the index
type IndexFile struct {
	// Files are stored in relative forward-slash format to the index file
	File           string `toml:"file"`
	Hash           string `toml:"hash"`
	HashFormat     string `toml:"hash-format,omitempty"`
	Alias          string `toml:"alias,omitempty"`
	MetaFile       bool   `toml:"metafile,omitempty"` // True when it is a .toml metadata file
	fileExistsTemp bool
}

// LoadIndex attempts to load the index file from a path
func LoadIndex(indexFile string, flags Flags) (Index, error) {
	data, err := ioutil.ReadFile(indexFile)
	if err != nil {
		return Index{}, err
	}
	var index Index
	if _, err := toml.Decode(string(data), &index); err != nil {
		return Index{}, err
	}
	index.flags = flags
	index.indexFile = indexFile
	if len(index.HashFormat) == 0 {
		index.HashFormat = "sha256"
	}
	return index, nil
}

// RemoveFile removes a file from the index.
func (in *Index) RemoveFile(path string) error {
	relPath, err := filepath.Rel(filepath.Dir(in.indexFile), path)
	if err != nil {
		return err
	}

	i := 0
	for _, file := range in.Files {
		if filepath.Clean(filepath.FromSlash(file.File)) != relPath {
			// Keep file, as it doesn't match
			in.Files[i] = file
			i++
		}
	}
	in.Files = in.Files[:i]
	return nil
}

// resortIndex sorts Files by file name
func (in *Index) resortIndex() {
	sort.SliceStable(in.Files, func(i, j int) bool {
		// TODO: Compare by alias if names are equal?
		// TODO: Remove duplicated entries? (compound key on file/alias?)
		return in.Files[i].File < in.Files[j].File
	})
}

// updateFile calculates the hash for a given path and updates it in the index
func (in *Index) updateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Hash usage strategy (may change):
	// Just use SHA256, overwrite existing hash regardless of what it is
	// May update later to continue using the same hash that was already being used
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	hashString := hex.EncodeToString(h.Sum(nil))

	// Find in index
	found := false
	relPath, err := filepath.Rel(filepath.Dir(in.indexFile), path)
	if err != nil {
		return err
	}
	for k, v := range in.Files {
		if filepath.Clean(filepath.FromSlash(v.File)) == relPath {
			found = true
			// Update hash
			in.Files[k].Hash = hashString
			if in.HashFormat == "sha256" {
				in.Files[k].HashFormat = ""
			} else {
				in.Files[k].HashFormat = "sha256"
			}
			// Mark this file as found
			in.Files[k].fileExistsTemp = true
			// Clean up path if it's untidy
			in.Files[k].File = filepath.ToSlash(relPath)
			// Don't break out of loop, as there may be aliased versions that
			// also need to be updated
		}
	}
	if !found {
		newFile := IndexFile{
			File:           filepath.ToSlash(relPath),
			Hash:           hashString,
			fileExistsTemp: true,
		}
		// Override hash format for this file, if the whole index isn't sha256
		if in.HashFormat != "sha256" {
			newFile.HashFormat = "sha256"
		}
		// If the file is in the mods folder, set MetaFile to true (mods are metafiles by default)
		// This is incredibly powerful: you can put a normal jar in the mods folder just by
		// setting MetaFile to false. Or you can use the "mod" metadata system for other types
		// of files, like CraftTweaker resources.
		absFileDir, err := filepath.Abs(filepath.Dir(path))
		if err == nil {
			absModsDir, err := filepath.Abs(in.flags.ModsFolder)
			if err == nil {
				if absFileDir == absModsDir {
					newFile.MetaFile = true
				}
			}
		}

		in.Files = append(in.Files, newFile)
	}

	return nil
}

// Refresh updates the hashes of all the files in the index, and adds new files to the index
func (in *Index) Refresh() error {
	// TODO: If needed, multithreaded hashing
	// for i := 0; i < runtime.NumCPU(); i++ {}

	// Get fileinfos of pack.toml and index to compare them
	pathPF, _ := filepath.Abs(in.flags.PackFile)
	pathIndex, _ := filepath.Abs(in.indexFile)

	// TODO: A method of specifying pack root directory?
	// TODO: A method of excluding files
	packRoot := filepath.Dir(in.flags.PackFile)
	err := filepath.Walk(packRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// TODO: Handle errors on individual files properly
			return err
		}
		// Exit if the files are the same as the pack/index files
		absPath, _ := filepath.Abs(path)
		if absPath == pathPF || absPath == pathIndex {
			return nil
		}
		// Exit if this is a directory
		if info.IsDir() {
			return nil
		}

		return in.updateFile(path)
	})
	if err != nil {
		return err
	}

	// Check all the files exist, remove them if they don't
	i := 0
	for _, file := range in.Files {
		if file.fileExistsTemp {
			// Keep file if it exists (already checked in updateFile)
			in.Files[i] = file
			i++
		}
	}
	in.Files = in.Files[:i]

	in.resortIndex()
	return nil
}

// Write saves the index file
func (in Index) Write() error {
	f, err := os.Create(in.indexFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	// Disable indentation
	enc.Indent = ""
	return enc.Encode(in)
}

