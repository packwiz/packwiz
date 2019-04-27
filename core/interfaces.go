package core
// UpdateParsers stores all the update parsers that packwiz can use. Add your own update systems to this map.
var UpdateParsers map[string]UpdateParser = make(map[string]UpdateParser)

// UpdateParser takes an unparsed interface{}, and returns an Updater for a mod file
type UpdateParser interface {
	ParseUpdate(interface{}) (Updater, error)
}

// Updater checks for and does updates on a mod
type Updater interface {
	// DoUpdate returns true if an update was done, false otherwise
	DoUpdate(Mod) (bool, error)
}

