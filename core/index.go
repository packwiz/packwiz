package core
import (
	"io/ioutil"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
)

// Index is a representation of the index.toml file for referencing all the files in a pack.
type Index struct {
	HashFormat string `toml:"hash-format"`
	Files      []struct {
		File       string `toml:"file"`
		Hash       string `toml:"hash"`
		HashFormat string `toml:"hash-format,omitempty"`
		Alias      string `toml:"alias,omitempty"`
	} `toml:"files"`
	flags     Flags
	indexFile string
}

// LoadIndex loads the index file
func LoadIndex(flags Flags) (Index, error) {
	indexFile, err := ResolveIndex(flags)
	if err != nil {
		return Index{}, err
	}

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
	return index, nil
}

// RemoveFile removes a file from the index.
func (in Index) RemoveFile(path string) {
	newFiles := in.Files[:0]
	for _, v := range in.Files {
		if v.File != path {
			newFiles = append(newFiles, v)
		}
	}
	in.Files = newFiles
}

// resortIndex sorts Files by file name
func (in Index) resortIndex() {
	sort.SliceStable(in.Files, func(i, j int) bool {
		// Compare by alias if names are equal?
		// Remove duplicated entries? (compound key on file/alias?)
		return in.Files[i].File < in.Files[j].File
	})
}

// Refresh updates the hashes of all the files in the index, and adds new files to the index
func (in Index) Refresh() error {
	// TODO: implement
	// process:
	// enumerate files, exclude index and pack.toml
	// hash them
	// check if they exist in list
	// if exists, modify existing entry(ies)
	// if not exists, add new entry
	// resort
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

