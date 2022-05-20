package core

import "io"

// Updaters stores all the updaters that packwiz can use. Add your own update systems to this map, keyed by the configuration name.
var Updaters = make(map[string]Updater)

// Updater is used to process updates on mods
type Updater interface {
	// ParseUpdate takes an unparsed interface{} (as a map[string]interface{}), and returns an Updater for a mod file.
	// This can be done using the mapstructure library or your own parsing methods.
	ParseUpdate(map[string]interface{}) (interface{}, error)
	// CheckUpdate checks whether there is an update for each of the mods in the given slice, for the given MC version,
	// called for all of the mods that this updater handles
	CheckUpdate([]Mod, string, Pack) ([]UpdateCheck, error)
	// DoUpdate carries out the update previously queried in CheckUpdate, on each Mod's metadata,
	// given pointers to Mods and the value of CachedState for each mod
	DoUpdate([]*Mod, []interface{}) error
}

// UpdateCheck represents the data returned from CheckUpdate for each mod
type UpdateCheck struct {
	// UpdateAvailable is true if an update is available for this mod
	UpdateAvailable bool
	// UpdateString is a string that details the update in some way to the user. Usually this will be in the form of
	// a version change (1.0.0 -> 1.0.1), or a file name change (thanos-skin-1.0.0.jar -> thanos-skin-1.0.1.jar).
	UpdateString string
	// CachedState can be used to preserve per-mod state between CheckUpdate and DoUpdate (e.g. file metadata)
	CachedState interface{}
	// Error stores an error for this specific mod
	// Errors can also be returned from CheckUpdate directly, if the whole operation failed completely (so only 1 error is printed)
	// If an error is returned for a mod, or from CheckUpdate, DoUpdate is not called on that mod / at all
	Error error
}

// MetaDownloaders stores all the metadata-based installers that packwiz can use. Add your own downloaders to this map, keyed by the source name.
var MetaDownloaders = make(map[string]MetaDownloader)

// MetaDownloader specifies a downloader for a Mod using a "metadata:source" mode
// The calling code should handle caching and hash validation.
type MetaDownloader interface {
	GetFilesMetadata([]*Mod) ([]MetaDownloaderData, error)
}

// MetaDownloaderData specifies the per-Mod metadata retrieved for downloading
type MetaDownloaderData interface {
	GetManualDownload() (bool, ManualDownload)
	DownloadFile() (io.ReadCloser, error)
}

type ManualDownload struct {
	Name     string
	FileName string
	URL      string
}
