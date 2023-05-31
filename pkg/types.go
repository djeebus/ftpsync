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
	Exists(path string) (bool, error)
	Write(path string, fp io.ReadCloser) (int64, error)
}

type Database interface {
	Exists(path string) (bool, error)
	Record(path, jobID string) error
}
