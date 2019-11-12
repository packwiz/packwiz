package packinterop

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type diskFile struct {
	NameInternal string
	Path         string
}

func (f diskFile) Name() string {
	return f.NameInternal
}

func (f diskFile) Open() (io.ReadCloser, error) {
	return os.Open(f.Path)
}

type readerFile struct {
	NameInternal string
	Reader       *io.ReadCloser
}

func (f readerFile) Name() string {
	return f.NameInternal
}

func (f readerFile) Open() (io.ReadCloser, error) {
	return *f.Reader, nil
}

type diskPackSource struct {
	MetaSource *bufio.Reader
	MetaName   string
	BasePath   string
}

func (s diskPackSource) GetFile(path string) (ImportPackFile, error) {
	return diskFile{path, filepath.Join(s.BasePath, filepath.FromSlash(path))}, nil
}

func (s diskPackSource) GetFileList() ([]ImportPackFile, error) {
	list := make([]ImportPackFile, 0)
	err := filepath.Walk(s.BasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Get the name of the file, relative to the pack folder
		name, err := filepath.Rel(s.BasePath, path)
		if err != nil {
			return err
		}
		list = append(list, diskFile{filepath.ToSlash(name), path})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s diskPackSource) GetPackFile() ImportPackFile {
	rc := ioutil.NopCloser(s.MetaSource)
	return readerFile{s.MetaName, &rc}
}

func GetDiskPackSource(metaSource *bufio.Reader, metaName string, basePath string) ImportPackSource {
	return diskPackSource{
		MetaSource: metaSource,
		MetaName:   metaName,
		BasePath:   basePath,
	}
}
