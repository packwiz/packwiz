package core
// Index is a representation of the index.toml file for referencing all the files in a pack.
type Index struct {
	HashFormat string `toml:"hash-format"`
	Files      []struct {
		File       string `toml:"file"`
		Hash       string `toml:"hash"`
		HashFormat string `toml:"hash-format,omitempty"`
		Alias      string `toml:"alias,omitempty"`
	} `toml:"files"`
}

// LoadIndex loads the index file
func LoadIndex(flags Flags) (Index, error) {
	indexFile, err := ResolveIndex(flags)
	if err != nil {
		return Index{}, err
	}

	_ = indexFile // TODO finish
	return Index{}, nil
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

