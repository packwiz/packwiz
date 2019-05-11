package core
// UpdateParsers stores all the update parsers that packwiz can use. Add your own update systems to this map.
var UpdateParsers = make(map[string]UpdateParser)

// UpdateParser takes an unparsed interface{} (as a map[string]interface{}), and returns an Updater for a mod file.
// This can be done using the mapstructure library or your own parsing methods.
type UpdateParser interface {
	ParseUpdate(map[string]interface{}) (Updater, error)
}

// Updater checks for and does updates on a mod
type Updater interface {
	// DoUpdate returns true if an update was done, false otherwise
	DoUpdate(Mod) (bool, error)
}

