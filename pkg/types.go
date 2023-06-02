package pkg

import "io"

type Source interface {
	List(path string) (ListResult, error)
	Read(path string) (io.ReadCloser, error)
}

type ListResult struct {
	Files   []string
	Folders []string
}

type Destination interface {
	GetAllFiles(path string) (*Set, error)
	Delete(path string) error
	Exists(path string) (bool, error)
	Write(path string, fp io.ReadCloser) (int64, error)
}

type Database interface {
	GetAllFiles() (*Set, error)
	Exists(path string) (bool, error)
	Record(path, jobID string) error
	Delete(path string) error
}
