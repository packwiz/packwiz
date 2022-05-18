package core

import (
	"encoding/json"
	"fmt"
	"golang.org/x/exp/slices"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type DownloadSession interface {
	GetManualDownloads() []ManualDownload
	StartDownloads(workers int) chan CompletedDownload
}

type CompletedDownload struct {
	File         *os.File
	DestFilePath string
	Hashes       map[string]string
	// Error indicates if/why downloading this file failed
	Error error
	// Warning indicates a message to show to the user regarding this file (download was successful, but had a problem)
	Warning error
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

func (d downloadSessionInternal) GetManualDownloads() []ManualDownload {
	return d.manualDownloads
}

func (d downloadSessionInternal) StartDownloads(workers int) chan CompletedDownload {
	tasks := make(chan downloadTask)
	downloads := make(chan CompletedDownload)
	var indexLock sync.RWMutex
	for i := 0; i < workers; i++ {
		go func() {
			for task := range tasks {
				// Lookup file in index
				indexLock.RLock()
				// Map hash stored in mod to cache hash format
				storedHashFmtList, hasStoredHashFmt := d.cacheIndex.Hashes[task.hashFormat]
				cacheHashFmtList := d.cacheIndex.Hashes[cacheHashFormat]
				if hasStoredHashFmt {
					hashIdx := slices.Index(storedHashFmtList, task.hash)
					if hashIdx > -1 {
						// Found in index; try using it!
						cacheFileHash := cacheHashFmtList[hashIdx]
						cacheFilePath := filepath.Join(d.cacheFolder, cacheFileHash[:2], cacheFileHash[2:])

						// Find hashes already stored in the index
						hashes := make(map[string]string)
						hashesToObtain := slices.Clone(d.hashesToObtain)
						for hashFormat, hashList := range d.cacheIndex.Hashes {
							if len(hashList) > hashIdx {
								hashes[hashFormat] = hashList[hashIdx]
							}
						}

						indexLock.RUnlock()

						// Assuming the file already exists, attempt to open it
						file, err := os.Open(cacheFilePath)
						if err == nil {
							// Calculate hashes
							if len(hashesToObtain) > 0 {
								// TODO: this code needs to add more hashes to the index
								err = teeHashes(cacheFileHash, cacheHashFormat, d.hashesToObtain, hashes, io.Discard, file)
								if err != nil {
									downloads <- CompletedDownload{
										Error: fmt.Errorf("failed to read hashes of file %s from cache: %w", cacheFilePath, err),
									}
									continue
								}
							}

							downloads <- CompletedDownload{
								File:         file,
								DestFilePath: task.destFilePath,
								Hashes:       hashes,
							}
							continue
						} else if !os.IsNotExist(err) {
							// Some other error trying to open the file!
							downloads <- CompletedDownload{
								Error: fmt.Errorf("failed to read file %s from cache: %w", cacheFilePath, err),
							}
							continue
						}
					}
				}
				indexLock.RUnlock()

				// Create temp file to download to
				tempFile, err := ioutil.TempFile(filepath.Join(d.cacheFolder, "temp"), "download-tmp")
				if err != nil {
					downloads <- CompletedDownload{
						Error: fmt.Errorf("failed to create temporary file for download: %w", err),
					}
					continue
				}

				hashes := make(map[string]string)
				hashes[task.hashFormat] = task.hash

				// TODO: do download
				var file *os.File
				indexLock.Lock()
				// Update hashes in the index and open file
				hashIdx := slices.Index(cacheHashFmtList, hashes[cacheHashFormat])
				if hashIdx < 0 {
					// Doesn't exist in the index; add as a new value
					hashIdx = len(cacheHashFmtList)

					cacheFileHash := cacheHashFmtList[hashIdx]
					cacheFilePath := filepath.Join(d.cacheFolder, cacheFileHash[:2], cacheFileHash[2:])
					// Create the containing directory
					err = os.MkdirAll(filepath.Dir(cacheFilePath), 0755)
					if err != nil {
						_ = tempFile.Close()
						indexLock.Unlock()
						downloads <- CompletedDownload{
							Error: fmt.Errorf("failed to create directories for file %s in cache: %w", cacheFilePath, err),
						}
						continue
					}
					// Create destination file
					file, err = os.Create(cacheFilePath)
					if err != nil {
						_ = tempFile.Close()
						indexLock.Unlock()
						downloads <- CompletedDownload{
							Error: fmt.Errorf("failed to write file %s to cache: %w", cacheFilePath, err),
						}
						continue
					}
					// Seek back to start of temp file
					_, err = tempFile.Seek(0, 0)
					if err != nil {
						_ = file.Close()
						_ = tempFile.Close()
						indexLock.Unlock()
						downloads <- CompletedDownload{
							Error: fmt.Errorf("failed to seek temp file %s in cache: %w", tempFile.Name(), err),
						}
						continue
					}
					// Copy temporary file to cache
					_, err = io.Copy(file, tempFile)
					if err != nil {
						_ = file.Close()
						_ = tempFile.Close()
						indexLock.Unlock()
						downloads <- CompletedDownload{
							Error: fmt.Errorf("failed to seek temp file %s in cache: %w", tempFile.Name(), err),
						}
						continue
					}
				} else {
					// Exists in the index and should exist on disk; open for reading
					cacheFileHash := cacheHashFmtList[hashIdx]
					cacheFilePath := filepath.Join(d.cacheFolder, cacheFileHash[:2], cacheFileHash[2:])
					file, err = os.Open(cacheFilePath)
					if err != nil {
						_ = tempFile.Close()
						indexLock.Unlock()
						downloads <- CompletedDownload{
							Error: fmt.Errorf("failed to write file %s to cache: %w", cacheFilePath, err),
						}
						continue
					}
				}
				// Close temporary file, as we are done with it
				err = tempFile.Close()
				if err != nil {
					_ = file.Close()
					indexLock.Unlock()
					downloads <- CompletedDownload{
						Error: fmt.Errorf("failed to close temporary file for download: %w", err),
					}
					continue
				}
				var warning error
				for hashFormat, hashList := range d.cacheIndex.Hashes {
					if hashIdx >= len(hashList) {
						// Add empty values to make hashList fit hashIdx
						hashList = append(hashList, make([]string, (hashIdx-len(hashList))+1)...)
						d.cacheIndex.Hashes[hashFormat] = hashList
					}
					// Replace if it doesn't already exist
					if hashList[hashIdx] == "" {
						hashList[hashIdx] = hashes[hashFormat]
					} else if hash, ok := hashes[hashFormat]; ok && hashList[hashIdx] != hash {
						// Warn if the existing hash is inconsistent!
						warning = fmt.Errorf("inconsistent %s hash for %s overwritten - value %s (expected %s)", hashFormat,
							file.Name(), hashList[hashIdx], hash)
						hashList[hashIdx] = hashes[hashFormat]
					}
				}
				indexLock.Unlock()

				downloads <- CompletedDownload{
					File:         file,
					DestFilePath: task.destFilePath,
					Hashes:       hashes,
					Warning:      warning,
				}
			}
		}()
	}
	go func() {
		for _, v := range d.downloadTasks {
			tasks <- v
		}
	}()
	return downloads
}

func teeHashes(validateHash string, validateHashFormat string, hashesToObtain []string, hashes map[string]string,
	dst io.Writer, src io.Reader) error {
	// TODO: implement
}

const cacheHashFormat = "sha256"

type CacheIndex struct {
	Version uint32
	Hashes  map[string][]string
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

	return downloadSession, nil
}
