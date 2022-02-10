package packinterop

import "io"

type ImportPackFile interface {
	Name() string
	Open() (io.ReadCloser, error)
}

type ImportPackMetadata interface {
	Name() string
	PackAuthor() string
	PackVersion() string
	Versions() map[string]string
	Mods() []AddonFileReference
	GetFiles() ([]ImportPackFile, error)
}

type ImportPackSource interface {
	GetFile(path string) (ImportPackFile, error)
	GetFileList() ([]ImportPackFile, error)
	GetPackFile() ImportPackFile
}
