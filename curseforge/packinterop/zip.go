package packinterop

import (
	"archive/zip"
	"errors"
)

type zipReaderFile struct {
	NameInternal string
	*zip.File
}

func (f zipReaderFile) Name() string {
	return f.NameInternal
}

type zipPackSource struct {
	MetaFile       *zip.File
	Reader         *zip.Reader
	cachedFileList []ImportPackFile
}

func (s zipPackSource) GetFile(path string) (ImportPackFile, error) {
	if s.cachedFileList == nil {
		s.cachedFileList = make([]ImportPackFile, len(s.Reader.File))
		for i, v := range s.Reader.File {
			s.cachedFileList[i] = zipReaderFile{v.Name, v}
		}
	}
	for _, v := range s.cachedFileList {
		if v.Name() == path {
			return v, nil
		}
	}
	return zipReaderFile{}, errors.New("file not found in zip")
}

func (s zipPackSource) GetFileList() ([]ImportPackFile, error) {
	if s.cachedFileList == nil {
		s.cachedFileList = make([]ImportPackFile, len(s.Reader.File))
		for i, v := range s.Reader.File {
			s.cachedFileList[i] = zipReaderFile{v.Name, v}
		}
	}
	return s.cachedFileList, nil
}

func (s zipPackSource) GetPackFile() ImportPackFile {
	return zipReaderFile{s.MetaFile.Name, s.MetaFile}
}

func GetZipPackSource(metaFile *zip.File, reader *zip.Reader) ImportPackSource {
	return zipPackSource{
		MetaFile: metaFile,
		Reader:   reader,
	}
}
