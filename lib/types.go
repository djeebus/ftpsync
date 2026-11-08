package lib

import "io"

type Source interface {
	// Read opens path starting at offset. It returns the offset the stream
	// actually starts at, which is 0 if the source could not honor the offset.
	Read(path string, offset int64) (io.ReadCloser, int64, error)
	GetAllFiles(path string) (*SizeSet, error)
	Close() error
}

type Precheck interface {
	IsFileReady(path string) (bool, error)
	Close() error
}

type Destination interface {
	GetAllFiles(path string) (*SizeSet, error)
	Delete(path string) error
	Exists(path string) (bool, error)
	// PartialSize returns how many bytes of an interrupted download of path are on disk.
	PartialSize(path string) (int64, error)
	// Write appends fp to the partial download of path, starting at offset, and
	// moves it into place once fp is exhausted. It returns the total file size.
	// On failure the partial download is kept so it can be resumed.
	Write(path string, fp io.Reader, offset int64) (int64, error)
	CleanDirectories(path string) error
}

type Database interface {
	GetAllFiles(path string) (*Set, error)
	Exists(path string) (bool, error)
	Record(path string) error
	Delete(path string) error
	Close() error
}
