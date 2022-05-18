package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/exp/slices"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type DownloadSession interface {
	GetManualDownloads() []ManualDownload
	StartDownloads() chan CompletedDownload
	SaveIndex() error
}

type CompletedDownload struct {
	File         *os.File
	DestFilePath string
	Hashes       map[string]string
	// Error indicates if/why downloading this file failed
	Error error
	// Warnings indicates messages to show to the user regarding this file (download was successful, but had a problem)
	Warnings []error
}

type downloadSessionInternal struct {
	cacheIndex      CacheIndex
	cacheFolder     string
	hashesToObtain  []string
	manualDownloads []ManualDownload
	downloadTasks   []downloadTask
}

type downloadTask struct {
	metaDownloaderData MetaDownloaderData
	destFilePath       string
	url                string
	hashFormat         string
	hash               string
}

func (d *downloadSessionInternal) GetManualDownloads() []ManualDownload {
	// TODO: set destpaths
	return d.manualDownloads
}

func (d *downloadSessionInternal) StartDownloads() chan CompletedDownload {
	downloads := make(chan CompletedDownload)
	for _, task := range d.downloadTasks {
		// Get handle for mod
		cacheHandle := d.cacheIndex.GetHandleFromHash(task.hashFormat, task.hash)
		if cacheHandle != nil {
			download, err := reuseExistingFile(cacheHandle, d.hashesToObtain, task.destFilePath)
			if err != nil {
				downloads <- CompletedDownload{
					Error: err,
				}
			} else {
				downloads <- download
			}
			continue
		}

		download, err := downloadNewFile(&task, d.cacheFolder, d.hashesToObtain, &d.cacheIndex)
		if err != nil {
			downloads <- CompletedDownload{
				Error: err,
			}
		} else {
			downloads <- download
		}
	}
	return downloads
}

func (d *downloadSessionInternal) SaveIndex() error {
	data, err := json.Marshal(d.cacheIndex)
	if err != nil {
		return fmt.Errorf("failed to serialise index: %w", err)
	}
	err = ioutil.WriteFile(filepath.Join(d.cacheFolder, "index.json"), data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}
	return nil
}

func reuseExistingFile(cacheHandle *CacheIndexHandle, hashesToObtain []string, destFilePath string) (CompletedDownload, error) {
	// Already stored; try using it!
	file, err := cacheHandle.Open()
	if err == nil {
		remainingHashes := cacheHandle.GetRemainingHashes(hashesToObtain)
		if len(remainingHashes) > 0 {
			err = teeHashes(remainingHashes, cacheHandle.Hashes, io.Discard, file)
			if err != nil {
				_ = file.Close()
				return CompletedDownload{}, fmt.Errorf("failed to read hashes of file %s from cache: %w", cacheHandle.Path(), err)
			}
			_, err := file.Seek(0, 0)
			if err != nil {
				_ = file.Close()
				return CompletedDownload{}, fmt.Errorf("failed to seek file %s in cache: %w", cacheHandle.Path(), err)
			}
			cacheHandle.UpdateIndex()
		}

		return CompletedDownload{
			File:         file,
			DestFilePath: destFilePath,
			Hashes:       cacheHandle.Hashes,
		}, nil
	} else {
		return CompletedDownload{}, fmt.Errorf("failed to read file %s from cache: %w", cacheHandle.Path(), err)
	}
}

func downloadNewFile(task *downloadTask, cacheFolder string, hashesToObtain []string, index *CacheIndex) (CompletedDownload, error) {
	// Create temp file to download to
	tempFile, err := ioutil.TempFile(filepath.Join(cacheFolder, "temp"), "download-tmp")
	if err != nil {
		return CompletedDownload{}, fmt.Errorf("failed to create temporary file for download: %w", err)
	}

	hashesToObtain, hashes := getHashListsForDownload(hashesToObtain, task.hashFormat, task.hash)
	if len(hashesToObtain) > 0 {
		var data io.ReadCloser
		if task.url != "" {
			resp, err := http.Get(task.url)
			// TODO: content type, user-agent?
			if err != nil {
				return CompletedDownload{}, fmt.Errorf("failed to download %s: %w", task.url, err)
			}
			if resp.StatusCode != 200 {
				_ = resp.Body.Close()
				return CompletedDownload{}, fmt.Errorf("failed to download %s: invalid status code %v", task.url, resp.StatusCode)
			}
			data = resp.Body
		} else {
			data, err = task.metaDownloaderData.DownloadFile()
			if err != nil {
				return CompletedDownload{}, err
			}
		}

		err = teeHashes(hashesToObtain, hashes, tempFile, data)
		_ = data.Close()
		if err != nil {
			return CompletedDownload{}, fmt.Errorf("failed to download file for %s: %w", task.destFilePath, err)
		}
	}

	// Create handle with calculated hashes
	cacheHandle, alreadyExists := index.NewHandleFromHashes(hashes)
	// Update index stored hashes
	warnings := cacheHandle.UpdateIndex()

	var file *os.File
	if alreadyExists {
		err = tempFile.Close()
		if err != nil {
			return CompletedDownload{}, fmt.Errorf("failed to close temporary file %s: %w", tempFile.Name(), err)
		}
		file, err = cacheHandle.Open()
		if err != nil {
			return CompletedDownload{}, fmt.Errorf("failed to read file %s from cache: %w", cacheHandle.Path(), err)
		}
	} else {
		// Automatically closes tempFile
		file, err = cacheHandle.CreateFromTemp(tempFile)
		if err != nil {
			_ = tempFile.Close()
			return CompletedDownload{}, fmt.Errorf("failed to move file %s to cache: %w", cacheHandle.Path(), err)
		}
	}

	return CompletedDownload{
		File:         file,
		DestFilePath: task.destFilePath,
		Hashes:       hashes,
		Warnings:     warnings,
	}, nil
}

func selectPreferredHash(hashes map[string]string) (currHash string, currHashFormat string) {
	for _, hashFormat := range preferredHashList {
		if hash, ok := hashes[hashFormat]; ok {
			currHashFormat = hashFormat
			currHash = hash
		}
	}
	return
}

// getHashListsForDownload creates a hashes map with the given validate hash+format,
// ensures cacheHashFormat is in hashesToObtain (cloned+returned) and validateHashFormat isn't
func getHashListsForDownload(hashesToObtain []string, validateHashFormat string, validateHash string) ([]string, map[string]string) {
	hashes := make(map[string]string)
	hashes[validateHashFormat] = validateHash

	cl := []string{cacheHashFormat}
	for _, v := range hashesToObtain {
		if v != validateHashFormat && v != cacheHashFormat {
			cl = append(cl, v)
		}
	}
	return cl, hashes
}

func teeHashes(hashesToObtain []string, hashes map[string]string,
	dst io.Writer, src io.Reader) error {
	// Select the best hash from the hashes map to validate against
	validateHashFormat, validateHash := selectPreferredHash(hashes)
	if validateHashFormat == "" {
		return errors.New("failed to find preferred hash for file")
	}

	// Create writers for all the hashers
	mainHasher, err := GetHashImpl(validateHashFormat)
	if err != nil {
		return fmt.Errorf("failed to get hash format %s", validateHashFormat)
	}
	hashers := make(map[string]HashStringer, len(hashesToObtain))
	allWriters := make([]io.Writer, len(hashers))
	for i, v := range hashesToObtain {
		hashers[v], err = GetHashImpl(v)
		if err != nil {
			return fmt.Errorf("failed to get hash format %s", v)
		}
		allWriters[i] = hashers[v]
	}
	allWriters = append(allWriters, mainHasher, dst)

	// Copy source to all writers (all hashers and dst)
	w := io.MultiWriter(allWriters...)
	_, err = io.Copy(w, src)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	calculatedHash := mainHasher.HashToString(mainHasher.Sum(nil))

	// Check if the hash of the downloaded file matches the expected hash
	if calculatedHash != validateHash {
		return fmt.Errorf(
			"%s hash of downloaded file does not match with expected hash!\n download hash: %s\n expected hash: %s\n",
			validateHashFormat, calculatedHash, validateHash)
	}

	for hashFormat, v := range hashers {
		hashes[hashFormat] = v.HashToString(v.Sum(nil))
	}

	return nil
}

const cacheHashFormat = "sha256"

type CacheIndex struct {
	Version     uint32
	Hashes      map[string][]string
	cachePath   string
	nextHashIdx int
}

type CacheIndexHandle struct {
	index   *CacheIndex
	hashIdx int
	Hashes  map[string]string
}

func (c *CacheIndex) GetHandleFromHash(hashFormat string, hash string) *CacheIndexHandle {
	storedHashFmtList, hasStoredHashFmt := c.Hashes[hashFormat]
	if hasStoredHashFmt {
		hashIdx := slices.Index(storedHashFmtList, hash)
		if hashIdx > -1 {
			hashes := make(map[string]string)
			for curHashFormat, hashList := range c.Hashes {
				if hashIdx < len(hashList) && hashList[hashIdx] != "" {
					hashes[curHashFormat] = hashList[hashIdx]
				}
			}
			return &CacheIndexHandle{
				index:   c,
				hashIdx: hashIdx,
				Hashes:  hashes,
			}
		}
	}
	return nil
}

func (c *CacheIndex) NewHandleFromHashes(hashes map[string]string) (*CacheIndexHandle, bool) {
	for hashFormat, hash := range hashes {
		handle := c.GetHandleFromHash(hashFormat, hash)
		if handle != nil {
			// Add hashes to handle
			for hashFormat2, hash2 := range hashes {
				handle.Hashes[hashFormat2] = hash2
			}
			return handle, true
		}
	}
	i := c.nextHashIdx
	c.nextHashIdx += 1
	return &CacheIndexHandle{
		index:   c,
		hashIdx: i,
		Hashes:  hashes,
	}, false
}

func (h *CacheIndexHandle) GetRemainingHashes(hashesToObtain []string) []string {
	var remaining []string
	for _, hashFormat := range hashesToObtain {
		if _, ok := h.Hashes[hashFormat]; !ok {
			remaining = append(remaining, hashFormat)
		}
	}
	return remaining
}

func (h *CacheIndexHandle) Path() string {
	cacheFileHash := h.index.Hashes[cacheHashFormat][h.hashIdx]
	cacheFilePath := filepath.Join(h.index.cachePath, cacheFileHash[:2], cacheFileHash[2:])
	return cacheFilePath
}

func (h *CacheIndexHandle) Open() (*os.File, error) {
	return os.Open(h.Path())
}

func (h *CacheIndexHandle) CreateFromTemp(temp *os.File) (*os.File, error) {
	err := os.MkdirAll(filepath.Dir(h.Path()), 0755)
	if err != nil {
		return nil, err
	}
	err = os.Rename(temp.Name(), h.Path())
	if err != nil {
		return nil, err
	}
	err = temp.Close()
	if err != nil {
		return nil, err
	}
	return os.Open(h.Path())
}

func (h *CacheIndexHandle) UpdateIndex() (warnings []error) {
	// Add hashes to index
	for hashFormat, hash := range h.Hashes {
		hashList := h.index.Hashes[hashFormat]
		if h.hashIdx >= len(hashList) {
			// Add empty values to make hashList fit hashIdx
			hashList = append(hashList, make([]string, (h.hashIdx-len(hashList))+1)...)
			h.index.Hashes[hashFormat] = hashList
		}

		// Replace if it doesn't already exist
		if hashList[h.hashIdx] == "" {
			hashList[h.hashIdx] = h.Hashes[hashFormat]
		} else if hashList[h.hashIdx] != hash {
			// Warn if the existing hash is inconsistent!
			warnings = append(warnings, fmt.Errorf("inconsistent %s hash for %s overwritten - value %s (expected %s)",
				hashFormat, h.Path(), hashList[h.hashIdx], hash))
			hashList[h.hashIdx] = h.Hashes[hashFormat]
		}
	}
	return
}

func CreateDownloadSession(mods []*Mod, hashesToObtain []string) (DownloadSession, error) {
	// Load cache index
	cacheIndex := CacheIndex{Version: 1, Hashes: make(map[string][]string)}
	cachePath, err := GetPackwizCache()
	if err != nil {
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}
	err = os.MkdirAll(cachePath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	err = os.MkdirAll(filepath.Join(cachePath, "temp"), 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache temp directory: %w", err)
	}
	cacheIndexData, err := ioutil.ReadFile(filepath.Join(cachePath, "index.json"))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read cache index file: %w", err)
		}
	} else {
		err = json.Unmarshal(cacheIndexData, &cacheIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to read cache index file: %w", err)
		}
		if cacheIndex.Version > 1 {
			return nil, fmt.Errorf("cache index is too new (version %v)", cacheIndex.Version)
		}
	}

	// Ensure some parts of the index are initialised
	_, hasCacheHashFmt := cacheIndex.Hashes[cacheHashFormat]
	if !hasCacheHashFmt {
		cacheIndex.Hashes[cacheHashFormat] = make([]string, 0)
	}
	cacheIndex.cachePath = cachePath
	cacheIndex.nextHashIdx = len(cacheIndex.Hashes[cacheHashFormat])

	// TODO: move in/ files?

	// Create session
	downloadSession := downloadSessionInternal{
		cacheIndex:     cacheIndex,
		cacheFolder:    cachePath,
		hashesToObtain: hashesToObtain,
	}

	pendingMetadata := make(map[string][]*Mod)

	// Get necessary metadata for all files
	for _, mod := range mods {
		if mod.Download.Mode == "url" {
			downloadSession.downloadTasks = append(downloadSession.downloadTasks, downloadTask{
				destFilePath: mod.GetDestFilePath(),
				url:          mod.Download.URL,
				hashFormat:   mod.Download.HashFormat,
				hash:         mod.Download.Hash,
			})
		} else if strings.HasPrefix(mod.Download.Mode, "metadata:") {
			dlID := strings.TrimPrefix(mod.Download.Mode, "metadata:")
			pendingMetadata[dlID] = append(pendingMetadata[dlID], mod)
		} else {
			return nil, fmt.Errorf("unknown download mode %s for mod %s", mod.Download.Mode, mod.Name)
		}
	}

	for dlID, mods := range pendingMetadata {
		downloader, ok := MetaDownloaders[dlID]
		if !ok {
			return nil, fmt.Errorf("unknown download mode %s for mod %s", mods[0].Download.Mode, mods[0].Name)
		}
		meta, err := downloader.GetFilesMetadata(mods)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve %s files: %w", dlID, err)
		}
		for i, v := range mods {
			isManual, manualDownload := meta[i].GetManualDownload()
			if isManual {
				downloadSession.manualDownloads = append(downloadSession.manualDownloads, manualDownload)
			} else {
				downloadSession.downloadTasks = append(downloadSession.downloadTasks, downloadTask{
					destFilePath:       v.GetDestFilePath(),
					metaDownloaderData: meta[i],
					hashFormat:         v.Download.HashFormat,
					hash:               v.Download.Hash,
				})
			}
		}
	}

	// TODO: index housekeeping? i.e. remove deleted files, remove old files (LRU?)

	return &downloadSession, nil
}
