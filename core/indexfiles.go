package core

import (
	"path"
	"slices"
	"strings"
)

// IndexFiles are stored as a map of path -> (indexFile or alias -> indexFile)
// The latter is used for multiple copies with the same path but different alias
type IndexFiles map[string]IndexPathHolder

type IndexPathHolder interface {
	updateHash(hash string, format string)
	markFound()
	markMetaFile()
	markedFound() bool
	IsMetaFile() bool
}

// indexFile is a file in the index
type indexFile struct {
	// Files are stored in forward-slash format relative to the index file
	File       string `toml:"file"`
	Hash       string `toml:"hash,omitempty"`
	HashFormat string `toml:"hash-format,omitempty"`
	Alias      string `toml:"alias,omitempty"`
	MetaFile   bool   `toml:"metafile,omitempty"` // True when it is a .toml metadata file
	Preserve   bool   `toml:"preserve,omitempty"` // Don't overwrite the file when updating
	fileFound  bool
}

func (i *indexFile) updateHash(hash string, format string) {
	i.Hash = hash
	i.HashFormat = format
}

func (i *indexFile) markFound() {
	i.fileFound = true
}

func (i *indexFile) markMetaFile() {
	i.MetaFile = true
}

func (i *indexFile) markedFound() bool {
	return i.fileFound
}

func (i *indexFile) IsMetaFile() bool {
	return i.MetaFile
}

type indexFileMultipleAlias map[string]indexFile

func (i *indexFileMultipleAlias) updateHash(hash string, format string) {
	for k, v := range *i {
		v.updateHash(hash, format)
		(*i)[k] = v // Can't mutate map value in place
	}
}

// (indexFileMultipleAlias == map[string]indexFile)
func (i *indexFileMultipleAlias) markFound() {
	for k, v := range *i {
		v.markFound()
		(*i)[k] = v // Can't mutate map value in place
	}
}

func (i *indexFileMultipleAlias) markMetaFile() {
	for k, v := range *i {
		v.markMetaFile()
		(*i)[k] = v // Can't mutate map value in place
	}
}

func (i *indexFileMultipleAlias) markedFound() bool {
	for _, v := range *i {
		return v.markedFound()
	}
	panic("No entries in indexFileMultipleAlias")
}

func (i *indexFileMultipleAlias) IsMetaFile() bool {
	for _, v := range *i {
		return v.MetaFile
	}
	panic("No entries in indexFileMultipleAlias")
}

// updateFileEntry updates the hash of a file and marks as found; adding it if it doesn't exist
// This also sets metafile if markAsMetaFile is set
// This updates all existing aliassed variants of a file, but doesn't create new ones
func (f *IndexFiles) updateFileEntry(path string, format string, hash string, markAsMetaFile bool) {
	// Ensure map is non-nil
	if *f == nil {
		*f = make(IndexFiles)
	}
	// Fetch existing entry
	file, found := (*f)[path]
	if found {
		// Exists: update hash/format/metafile
		file.markFound()
		file.updateHash(hash, format)
		if markAsMetaFile {
			file.markMetaFile()
		}
		// (don't do anything if markAsMetaFile is false - don't reset metafile status of existing metafiles)
	} else {
		// Doesn't exist: create new file data
		newFile := indexFile{
			File:       path,
			Hash:       hash,
			HashFormat: format,
			MetaFile:   markAsMetaFile,
			fileFound:  true,
		}
		(*f)[path] = &newFile
	}
}

type indexFilesTomlRepresentation []indexFile

// toMemoryRep converts the TOML representation of IndexFiles to that used in memory
// These silly converter functions are necessary because the TOML libraries don't support custom non-primitive serializers
func (rep indexFilesTomlRepresentation) toMemoryRep() IndexFiles {
	out := make(IndexFiles)

	// Add entries to map
	for _, v := range rep {
		v := v // Narrow scope of loop variable
		v.File = path.Clean(v.File)
		v.Alias = path.Clean(v.Alias)
		// path.Clean converts "" into "." - undo this for Alias as we use omitempty
		if v.Alias == "." {
			v.Alias = ""
		}
		if existing, ok := out[v.File]; ok {
			if existingFile, ok := existing.(*indexFile); ok {
				// Is this the same as the existing file?
				if v.Alias == existingFile.Alias {
					// Yes: overwrite
					out[v.File] = &v
				} else {
					// No: convert to new map
					m := make(indexFileMultipleAlias)
					m[existingFile.Alias] = *existingFile
					m[v.Alias] = v
					out[v.File] = &m
				}
			} else if existingMap, ok := existing.(*indexFileMultipleAlias); ok {
				// Add to alias map
				(*existingMap)[v.Alias] = v
			} else {
				panic("Unknown type in IndexFiles")
			}
		} else {
			out[v.File] = &v
		}
	}

	return out
}

// toTomlRep converts the in-memory representation of IndexFiles to that used in TOML
// These silly converter functions are necessary because the TOML libraries don't support custom non-primitive serializers
func (f *IndexFiles) toTomlRep() indexFilesTomlRepresentation {
	// Turn internal representation into TOML representation
	rep := make(indexFilesTomlRepresentation, 0, len(*f))
	for _, v := range *f {
		if file, ok := v.(*indexFile); ok {
			rep = append(rep, *file)
		} else if file, ok := v.(*indexFileMultipleAlias); ok {
			for _, alias := range *file {
				rep = append(rep, alias)
			}
		} else {
			panic("Unknown type in IndexFiles")
		}
	}

	slices.SortFunc(rep, func(a indexFile, b indexFile) int {
		if a.File == b.File {
			return strings.Compare(a.Alias, b.Alias)
		} else {
			return strings.Compare(a.File, b.File)
		}
	})

	return rep
}
