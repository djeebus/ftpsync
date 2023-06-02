package lib

import "io"

type Source interface {
	Read(path string) (io.ReadCloser, error)
	GetAllFiles(path string) (*Set, error)
}

type Destination interface {
	GetAllFiles(path string) (*Set, error)
	Delete(path string) error
	Exists(path string) (bool, error)
	Write(path string, fp io.ReadCloser) (int64, error)
}

type Database interface {
	GetAllFiles(path string) (*Set, error)
	Exists(path string) (bool, error)
	Record(path string) error
	Delete(path string) error
}
