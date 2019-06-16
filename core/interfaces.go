package core

// Updaters stores all the updaters that packwiz can use. Add your own update systems to this map, keyed by the configuration name.
var Updaters = make(map[string]Updater)

// Updater is used to process updates on mods
type Updater interface {
	// ParseUpdate takes an unparsed interface{} (as a map[string]interface{}), and returns an Updater for a mod file.
	// This can be done using the mapstructure library or your own parsing methods.
	ParseUpdate(map[string]interface{}) (interface{}, error)
	// CheckUpdate checks whether there is an update for each of the mods in the given slice,
	// called for all of the mods that this updater handles
	CheckUpdate([]Mod) ([]UpdateCheck, error)
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

// TODO: new docs
// to carry out updating:

// go through all metafiles in index
// make a []Mod for each updater, so map[string][]Mod
// for each Mod, check the "first" updater, then give the Mod to the map

// go through the map, call CheckUpdate with the []Mod
// print to user, if interactive mode
// call doupdate with the mods and interfaces!!
