package packinterop

import "io"

type ImportPackFile interface {
	Name() string
	Open() (io.ReadCloser, error)
}

type ImportPackMetadata interface {
	Name() string
	Versions() map[string]string
	// TODO: use AddonFileReference?
	Mods() []struct {
		ModID  int
		FileID int
	}
	GetFiles() ([]ImportPackFile, error)
}

type ImportPackSource interface {
	GetFile(path string) (ImportPackFile, error)
	GetFileList() ([]ImportPackFile, error)
	GetPackFile() ImportPackFile
}
