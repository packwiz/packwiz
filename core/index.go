package core

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"
)

// Index is a representation of the index.toml file for referencing all the files in a pack.
type Index struct {
	HashFormat string      `toml:"hash-format"`
	Files      []IndexFile `toml:"files"`
	indexFile  string
}

// IndexFile is a file in the index
type IndexFile struct {
	// Files are stored in forward-slash format relative to the index file
	File           string `toml:"file"`
	Hash           string `toml:"hash,omitempty"`
	HashFormat     string `toml:"hash-format,omitempty"`
	Alias          string `toml:"alias,omitempty"`
	MetaFile       bool   `toml:"metafile,omitempty"` // True when it is a .toml metadata file
	Preserve       bool   `toml:"preserve,omitempty"` // Don't overwrite the file when updating
	fileExistsTemp bool
}

// LoadIndex attempts to load the index file from a path
func LoadIndex(indexFile string) (Index, error) {
	var index Index
	if _, err := toml.DecodeFile(indexFile, &index); err != nil {
		return Index{}, err
	}
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

func (in *Index) updateFileHashGiven(path, format, hash string, mod bool) error {
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
			in.Files[k].Hash = hash
			if in.HashFormat == format {
				in.Files[k].HashFormat = ""
			} else {
				in.Files[k].HashFormat = format
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
			Hash:           hash,
			fileExistsTemp: true,
		}
		// Override hash format for this file, if the whole index isn't sha256
		if in.HashFormat != format {
			newFile.HashFormat = format
		}
		newFile.MetaFile = mod

		in.Files = append(in.Files, newFile)
	}
	return nil
}

// updateFile calculates the hash for a given path and updates it in the index
func (in *Index) updateFile(path string) error {
	var hashString string
	if viper.GetBool("no-internal-hashes") {
		hashString = ""
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}

		// Hash usage strategy (may change):
		// Just use SHA256, overwrite existing hash regardless of what it is
		// May update later to continue using the same hash that was already being used
		h, err := GetHashImpl("sha256")
		if err != nil {
			_ = f.Close()
			return err
		}
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
		hashString = h.HashToString(h.Sum(nil))
	}

	mod := false
	// If the file has an extension of pw.toml, set mod to true
	if strings.HasSuffix(filepath.Base(path), MetaExtension) {
		mod = true
	}

	return in.updateFileHashGiven(path, "sha256", hashString, mod)
}

func (in Index) GetPackRoot() string {
	return filepath.Dir(in.indexFile)
}

var ignoreDefaults = []string{
	// Defaults (can be overridden with a negating pattern preceded with !)

	// Exclude Git metadata
	".git/**",
	".gitattributes",
	".gitignore",

	// Exclude exported CurseForge zip files
	"/*.zip",

	// Exclude exported Modrinth packs
	"*.mrpack",

	// Exclude packwiz binaries, if the user puts them in their pack folder
	"packwiz.exe",
	"packwiz", // Note: also excludes packwiz/ as a directory - you can negate this pattern if you want a directory called packwiz
}

func readGitignore(path string) (*gitignore.GitIgnore, bool) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		// TODO: check for read errors (and present them)
		return gitignore.CompileIgnoreLines(ignoreDefaults...), false
	}

	s := strings.Split(string(data), "\n")
	var lines []string
	lines = append(lines, ignoreDefaults...)
	lines = append(lines, s...)
	return gitignore.CompileIgnoreLines(lines...), true
}

// Refresh updates the hashes of all the files in the index, and adds new files to the index
func (in *Index) Refresh() error {
	// TODO: If needed, multithreaded hashing
	// for i := 0; i < runtime.NumCPU(); i++ {}

	// Is case-sensitivity a problem?
	pathPF, _ := filepath.Abs(viper.GetString("pack-file"))
	pathIndex, _ := filepath.Abs(in.indexFile)

	packRoot := in.GetPackRoot()
	ignoreExists := true
	pathIgnore, _ := filepath.Abs(filepath.Join(packRoot, ".packwizignore"))
	ignore, ignoreExists := readGitignore(filepath.Join(packRoot, ".packwizignore"))

	var fileList []string
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
		if ignoreExists {
			if absPath == pathIgnore {
				return nil
			}
		}
		if ignore.MatchesPath(path) {
			return nil
		}

		fileList = append(fileList, path)
		return nil
	})
	if err != nil {
		return err
	}

	progressContainer := mpb.New()
	progress := progressContainer.AddBar(int64(len(fileList)),
		mpb.PrependDecorators(
			// simple name decorator
			decor.Name("Refreshing index..."),
			// decor.DSyncWidth bit enables column width synchronization
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			// replace ETA decorator with "done" message, OnComplete event
			decor.OnComplete(
				// ETA decorator with ewma age of 60
				decor.EwmaETA(decor.ET_STYLE_GO, 60), "done",
			),
		),
	)

	for _, v := range fileList {
		start := time.Now()

		err := in.updateFile(v)
		if err != nil {
			return err
		}

		progress.Increment(time.Since(start))
	}
	// Close bar
	progress.SetTotal(int64(len(fileList)), true) // If len = 0, we have to manually set complete to true
	progressContainer.Wait()

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

// RefreshFile calculates the hash for a given path and updates it in the index (also sorts the index)
func (in *Index) RefreshFile(path string) error {
	err := in.updateFile(path)
	if err != nil {
		return err
	}
	in.resortIndex()
	return nil
}

// Write saves the index file
func (in Index) Write() error {
	// TODO: calculate and provide hash while writing?
	f, err := os.Create(in.indexFile)
	if err != nil {
		return err
	}

	enc := toml.NewEncoder(f)
	// Disable indentation
	enc.Indent = ""
	err = enc.Encode(in)
	if err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// RefreshFileWithHash updates a file in the index, given a file hash and whether it is a mod or not
func (in *Index) RefreshFileWithHash(path, format, hash string, mod bool) error {
	if viper.GetBool("no-internal-hashes") {
		hash = ""
	}
	err := in.updateFileHashGiven(path, format, hash, mod)
	if err != nil {
		return err
	}
	in.resortIndex()
	return nil
}

// FindMod finds a mod in the index and returns it's path and whether it has been found
func (in Index) FindMod(modName string) (string, bool) {
	for _, v := range in.Files {
		if v.MetaFile {
			_, file := filepath.Split(v.File)
			fileTrimmed := strings.TrimSuffix(strings.TrimSuffix(file, MetaExtension), MetaExtensionOld)
			if fileTrimmed == modName {
				return filepath.Join(filepath.Dir(in.indexFile), filepath.FromSlash(v.File)), true
			}
		}
	}
	return "", false
}

// GetAllMods finds paths to every metadata file (Mod) in the index
func (in Index) GetAllMods() []string {
	var list []string
	baseDir := filepath.Dir(in.indexFile)
	for _, v := range in.Files {
		if v.MetaFile {
			list = append(list, filepath.Join(baseDir, filepath.FromSlash(v.File)))
		}
	}
	return list
}

// LoadAllMods reads all metadata files into Mod structs
func (in Index) LoadAllMods() ([]*Mod, error) {
	modPaths := in.GetAllMods()
	mods := make([]*Mod, len(modPaths))
	for i, v := range modPaths {
		modData, err := LoadMod(v)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata file %s: %w", v, err)
		}
		mods[i] = &modData
	}
	return mods, nil
}

// GetFilePath attempts to get the path of the destination index file as it is stored on disk
func (in Index) GetFilePath(f IndexFile) string {
	return filepath.Join(filepath.Dir(in.indexFile), filepath.FromSlash(f.File))
}

// SaveFile attempts to read the file from disk
func (in Index) SaveFile(f IndexFile, dest io.Writer) error {
	hashFormat := f.HashFormat
	if hashFormat == "" {
		hashFormat = in.HashFormat
	}
	src, err := os.Open(in.GetFilePath(f))
	defer func(src *os.File) {
		_ = src.Close()
	}(src)
	if err != nil {
		return err
	}
	h, err := GetHashImpl(hashFormat)
	if err != nil {
		return err
	}

	w := io.MultiWriter(h, dest)
	_, err = io.Copy(w, src)
	if err != nil {
		return err
	}

	calculatedHash := h.HashToString(h.Sum(nil))
	if calculatedHash != f.Hash && !viper.GetBool("no-internal-hashes") {
		return errors.New("hash of saved file is invalid")
	}

	return nil
}
