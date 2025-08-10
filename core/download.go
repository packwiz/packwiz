package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"slices"
)

const UserAgent = "packwiz/packwiz"

func GetWithUA(url string, contentType string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", contentType)
	return http.DefaultClient.Do(req)
}

const DownloadCacheImportFolder = "import"

type DownloadSession interface {
	GetManualDownloads() []ManualDownload
	StartDownloads() chan CompletedDownload
	SaveIndex() error
}

type CompletedDownload struct {
	// File is only populated when the download is successful; points to the opened cache file
	File *os.File
	Mod  *Mod
	// Hashes is only populated when the download is successful; contains all stored hashes of the file
	Hashes map[string]string
	// Error indicates if/why downloading this file failed
	Error error
	// Warnings indicates messages to show to the user regarding this file (download was successful, but had a problem)
	Warnings []error
}

type downloadSessionInternal struct {
	cacheIndex           CacheIndex
	cacheFolder          string
	hashesToObtain       []string
	manualDownloads      []ManualDownload
	downloadTasks        []downloadTask
	foundManualDownloads []CompletedDownload
}

type downloadTask struct {
	metaDownloaderData MetaDownloaderData
	mod                *Mod
	url                string
	hashFormat         string
	hash               string
}

func (d *downloadSessionInternal) GetManualDownloads() []ManualDownload {
	return d.manualDownloads
}

func (d *downloadSessionInternal) StartDownloads() chan CompletedDownload {
	downloads := make(chan CompletedDownload)
	go func() {
		for _, found := range d.foundManualDownloads {
			downloads <- found
		}
		for _, task := range d.downloadTasks {
			warnings := make([]error, 0)

			// Get handle for mod
			cacheHandle := d.cacheIndex.GetHandleFromHash(task.hashFormat, task.hash)
			if cacheHandle != nil {
				download, err := reuseExistingFile(cacheHandle, d.hashesToObtain, task.mod)
				if err != nil {
					// Remove handle and try again
					cacheHandle.Remove()
					cacheHandle = nil
					warnings = append(warnings, fmt.Errorf("redownloading cached file: %w", err))
				} else {
					downloads <- download
					continue
				}
			}

			download, err := downloadNewFile(&task, d.cacheFolder, d.hashesToObtain, &d.cacheIndex)
			if err != nil {
				downloads <- CompletedDownload{
					Error: err,
					Mod:   task.mod,
				}
			} else {
				download.Warnings = warnings
				downloads <- download
			}
		}
		close(downloads)
	}()
	return downloads
}

func (d *downloadSessionInternal) SaveIndex() error {
	data, err := json.Marshal(d.cacheIndex)
	if err != nil {
		return fmt.Errorf("failed to serialise index: %w", err)
	}
	err = os.WriteFile(filepath.Join(d.cacheFolder, "index.json"), data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}
	return nil
}

func reuseExistingFile(cacheHandle *CacheIndexHandle, hashesToObtain []string, mod *Mod) (CompletedDownload, error) {
	// Already stored; try using it!
	file, err := cacheHandle.Open()
	if err == nil {
		remainingHashes := cacheHandle.GetRemainingHashes(hashesToObtain)
		var warnings []error
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
			warnings = cacheHandle.UpdateIndex()
		}

		return CompletedDownload{
			File:     file,
			Mod:      mod,
			Hashes:   cacheHandle.Hashes,
			Warnings: warnings,
		}, nil
	} else {
		return CompletedDownload{}, fmt.Errorf("failed to read file %s from cache: %w", cacheHandle.Path(), err)
	}
}

func downloadNewFile(task *downloadTask, cacheFolder string, hashesToObtain []string, index *CacheIndex) (CompletedDownload, error) {
	// Create temp file to download to
	tempFile, err := os.CreateTemp(filepath.Join(cacheFolder, "temp"), "download-tmp")
	if err != nil {
		return CompletedDownload{}, fmt.Errorf("failed to create temporary file for download: %w", err)
	}

	hashesToObtain, hashes := getHashListsForDownload(hashesToObtain, task.hashFormat, task.hash)
	if len(hashesToObtain) > 0 {
		var data io.ReadCloser
		if task.url != "" {
			resp, err := GetWithUA(task.url, "application/octet-stream")
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
			return CompletedDownload{}, fmt.Errorf("failed to download: %w", err)
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
		File:     file,
		Mod:      task.mod,
		Hashes:   hashes,
		Warnings: warnings,
	}, nil
}

func selectPreferredHash(hashes map[string]string) (currHashFormat string, currHash string) {
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

	var cl []string
	if cacheHashFormat != validateHashFormat {
		cl = append(cl, cacheHashFormat)
	}
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
	allWriters := make([]io.Writer, len(hashesToObtain))
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
	if strings.ToLower(calculatedHash) != strings.ToLower(validateHash) {
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

func (c *CacheIndex) getHashesMap(i int) map[string]string {
	hashes := make(map[string]string)
	for curHashFormat, hashList := range c.Hashes {
		if i < len(hashList) && hashList[i] != "" {
			hashes[curHashFormat] = hashList[i]
		}
	}
	return hashes
}

func (c *CacheIndex) GetHandleFromHash(hashFormat string, hash string) *CacheIndexHandle {
	storedHashFmtList, hasStoredHashFmt := c.Hashes[hashFormat]
	if hasStoredHashFmt {
		hashIdx := slices.Index(storedHashFmtList, strings.ToLower(hash))
		if hashIdx > -1 {
			return &CacheIndexHandle{
				index:   c,
				hashIdx: hashIdx,
				Hashes:  c.getHashesMap(hashIdx),
			}
		}
	}
	return nil
}

// GetHandleFromHashForce looks up the given hash in the index; but will rehash any file without this hash format to
// obtain the necessary hash. Only use this for manually downloaded files, as it can rehash every file in the cache, which
// can be more time-consuming than just redownloading the file and noticing it is already in the index!
func (c *CacheIndex) GetHandleFromHashForce(hashFormat string, hash string) (*CacheIndexHandle, error) {
	storedHashFmtList, hasStoredHashFmt := c.Hashes[hashFormat]
	if hasStoredHashFmt {
		// Ensure hash list is extended to the length of the cache hash format list
		storedHashFmtList = append(storedHashFmtList, make([]string, len(c.Hashes[cacheHashFormat])-len(storedHashFmtList))...)
		c.Hashes[hashFormat] = storedHashFmtList
		// Rehash every file that doesn't have this hash with this hash
		for hashIdx, curHash := range storedHashFmtList {
			if strings.EqualFold(curHash, hash) {
				return &CacheIndexHandle{
					index:   c,
					hashIdx: hashIdx,
					Hashes:  c.getHashesMap(hashIdx),
				}, nil
			} else if curHash == "" {
				var err error
				storedHashFmtList[hashIdx], err = c.rehashFile(c.Hashes[cacheHashFormat][hashIdx], hashFormat)
				if err != nil {
					return nil, fmt.Errorf("failed to rehash %s: %w", c.Hashes[cacheHashFormat][hashIdx], err)
				}
				if strings.EqualFold(storedHashFmtList[hashIdx], hash) {
					return &CacheIndexHandle{
						index:   c,
						hashIdx: hashIdx,
						Hashes:  c.getHashesMap(hashIdx),
					}, nil
				}
			}
		}
	} else {
		// Rehash every file with this hash
		storedHashFmtList = make([]string, len(c.Hashes[cacheHashFormat]))
		c.Hashes[hashFormat] = storedHashFmtList
		for hashIdx, cacheHash := range c.Hashes[cacheHashFormat] {
			var err error
			storedHashFmtList[hashIdx], err = c.rehashFile(cacheHash, hashFormat)
			if err != nil {
				return nil, fmt.Errorf("failed to rehash %s: %w", cacheHash, err)
			}
			if strings.EqualFold(storedHashFmtList[hashIdx], hash) {
				return &CacheIndexHandle{
					index:   c,
					hashIdx: hashIdx,
					Hashes:  c.getHashesMap(hashIdx),
				}, nil
			}
		}
	}
	return nil, nil
}

func (c *CacheIndex) rehashFile(cacheHash string, hashFormat string) (string, error) {
	file, err := os.Open(filepath.Join(c.cachePath, cacheHash[:2], cacheHash[2:]))
	if err != nil {
		return "", err
	}
	validateHasher, err := GetHashImpl(cacheHashFormat)
	if err != nil {
		return "", fmt.Errorf("failed to get hasher for rehash: %w", err)
	}
	rehashHasher, err := GetHashImpl(hashFormat)
	if err != nil {
		return "", fmt.Errorf("failed to get hasher for rehash: %w", err)
	}
	writer := io.MultiWriter(validateHasher, rehashHasher)
	_, err = io.Copy(writer, file)
	if err != nil {
		return "", err
	}

	validateHash := validateHasher.HashToString(validateHasher.Sum(nil))
	if cacheHash != validateHash {
		return "", fmt.Errorf(
			"%s hash of cached file does not match with expected hash!\n read hash: %s\n expected hash: %s\n",
			cacheHashFormat, validateHash, cacheHash)
	}
	return rehashHasher.HashToString(rehashHasher.Sum(nil)), nil
}

func (c *CacheIndex) NewHandleFromHashes(hashes map[string]string) (*CacheIndexHandle, bool) {
	// Ensure hashes contains the cache hash format
	if _, ok := hashes[cacheHashFormat]; !ok {
		panic("NewHandleFromHashes didn't get any value for " + cacheHashFormat)
	}
	// Only compare with the cache hash format - other hashes might be insecure or likely to collide
	handle := c.GetHandleFromHash(cacheHashFormat, hashes[cacheHashFormat])
	if handle != nil {
		// Add hashes to handle
		for hashFormat2, hash2 := range hashes {
			handle.Hashes[hashFormat2] = strings.ToLower(hash2)
		}
		return handle, true
	}
	i := c.nextHashIdx
	c.nextHashIdx += 1
	return &CacheIndexHandle{
		index:   c,
		hashIdx: i,
		Hashes:  hashes,
	}, false
}

func (c *CacheIndex) MoveImportFiles() error {
	return filepath.Walk(filepath.Join(c.cachePath, DownloadCacheImportFolder), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			_ = file.Close()
			return fmt.Errorf("failed to open imported file %s: %w", path, err)
		}
		hasher, err := GetHashImpl(cacheHashFormat)
		if err != nil {
			_ = file.Close()
			return fmt.Errorf("failed to validate imported file %s: %w", path, err)
		}
		_, err = io.Copy(hasher, file)
		if err != nil {
			_ = file.Close()
			return fmt.Errorf("failed to validate imported file %s: %w", path, err)
		}
		handle, exists := c.NewHandleFromHashes(map[string]string{
			cacheHashFormat: hasher.HashToString(hasher.Sum(nil)),
		})
		if exists {
			err = file.Close()
			if err != nil {
				return fmt.Errorf("failed to close imported file %s: %w", path, err)
			}
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf("failed to delete imported file %s: %w", path, err)
			}
		} else {
			newFile, err := handle.CreateFromTemp(file)
			if err != nil {
				if newFile != nil {
					_ = newFile.Close()
				}
				return fmt.Errorf("failed to rename imported file %s: %w", path, err)
			}
			err = newFile.Close()
			if err != nil {
				return fmt.Errorf("failed to close renamed imported file %s: %w", path, err)
			}
			_ = handle.UpdateIndex()
		}
		return nil
	})
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
	cacheFileHash := h.Hashes[cacheHashFormat]
	cacheFilePath := filepath.Join(h.index.cachePath, cacheFileHash[:2], cacheFileHash[2:])
	return cacheFilePath
}

func (h *CacheIndexHandle) Open() (*os.File, error) {
	return os.Open(h.Path())
}

func (h *CacheIndexHandle) CreateFromTemp(temp *os.File) (*os.File, error) {
	err := temp.Close()
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(h.Path()), 0755)
	if err != nil {
		return nil, err
	}
	err = os.Rename(temp.Name(), h.Path())
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

func (h *CacheIndexHandle) Remove() {
	for hashFormat := range h.Hashes {
		hashList := h.index.Hashes[hashFormat]
		if h.hashIdx < len(hashList) {
			h.index.Hashes[hashFormat] = slices.Delete(hashList, h.hashIdx, h.hashIdx+1)
		}
	}
	return
}

func removeIndices(hashList []string, indices []int) []string {
	i := 0
	for _, v := range hashList {
		if len(indices) > 0 && i == indices[0] {
			indices = indices[1:]
		} else {
			hashList[i] = v
			i++
		}
	}
	return hashList[:i]
}

func removeEmpty(hashList []string) ([]string, []int) {
	var indices []int
	i := 0
	for oldIdx, v := range hashList {
		if v == "" {
			indices = append(indices, oldIdx)
		} else {
			hashList[i] = v
			i++
		}
	}
	return hashList[:i], indices
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
	cacheIndexData, err := os.ReadFile(filepath.Join(cachePath, "index.json"))
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

	// Clean up empty entries in index
	var removedEntries []int
	cacheIndex.Hashes[cacheHashFormat], removedEntries = removeEmpty(cacheIndex.Hashes[cacheHashFormat])
	if len(removedEntries) > 0 {
		for hashFormat, v := range cacheIndex.Hashes {
			if hashFormat != cacheHashFormat {
				cacheIndex.Hashes[hashFormat] = removeIndices(v, removedEntries)
			}
		}
	}

	cacheIndex.nextHashIdx = len(cacheIndex.Hashes[cacheHashFormat])

	// Create import folder
	err = os.MkdirAll(filepath.Join(cachePath, DownloadCacheImportFolder), 0755)
	if err != nil {
		return nil, fmt.Errorf("error creating cache import folder: %w", err)
	}
	// Move import files
	err = cacheIndex.MoveImportFiles()
	if err != nil {
		return nil, fmt.Errorf("error updating cache import folder: %w", err)
	}

	// Create session
	downloadSession := downloadSessionInternal{
		cacheIndex:     cacheIndex,
		cacheFolder:    cachePath,
		hashesToObtain: hashesToObtain,
	}

	pendingMetadata := make(map[string][]*Mod)

	// Get necessary metadata for all files
	for _, mod := range mods {
		if mod.Download.Mode == ModeURL || mod.Download.Mode == "" {
			downloadSession.downloadTasks = append(downloadSession.downloadTasks, downloadTask{
				mod:        mod,
				url:        mod.Download.URL,
				hashFormat: mod.Download.HashFormat,
				hash:       mod.Download.Hash,
			})
		} else if strings.HasPrefix(mod.Download.Mode, "metadata:") {
			dlID := strings.TrimPrefix(mod.Download.Mode, "metadata:")
			pendingMetadata[dlID] = append(pendingMetadata[dlID], mod)
		} else {
			return nil, fmt.Errorf("unknown download mode %s for %s", mod.Download.Mode, mod.Name)
		}
	}

	for dlID, mods := range pendingMetadata {
		downloader, ok := MetaDownloaders[dlID]
		if !ok {
			return nil, fmt.Errorf("unknown download mode %s for %s", mods[0].Download.Mode, mods[0].Name)
		}
		meta, err := downloader.GetFilesMetadata(mods)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve %s files: %w", dlID, err)
		}
		for i, v := range mods {
			isManual, manualDownload := meta[i].GetManualDownload()
			if isManual {
				handle, err := cacheIndex.GetHandleFromHashForce(v.Download.HashFormat, v.Download.Hash)
				if err != nil {
					return nil, fmt.Errorf("failed to lookup manual download %s: %w", v.Name, err)
				}
				if handle != nil {
					file, err := handle.Open()
					if err != nil {
						return nil, fmt.Errorf("failed to open manual download %s: %w", v.Name, err)
					}
					downloadSession.foundManualDownloads = append(downloadSession.foundManualDownloads, CompletedDownload{
						File:   file,
						Mod:    v,
						Hashes: handle.Hashes,
					})
				} else {
					downloadSession.manualDownloads = append(downloadSession.manualDownloads, manualDownload)
				}
			} else {
				downloadSession.downloadTasks = append(downloadSession.downloadTasks, downloadTask{
					mod:                v,
					metaDownloaderData: meta[i],
					hashFormat:         v.Download.HashFormat,
					hash:               v.Download.Hash,
				})
			}
		}
	}

	// TODO: index housekeeping? i.e. remove deleted files, remove old files (LRU?)

	// Save index after importing and Force index updates
	err = downloadSession.SaveIndex()
	if err != nil {
		return nil, fmt.Errorf("error writing cache index: %w", err)
	}

	return &downloadSession, nil
}
