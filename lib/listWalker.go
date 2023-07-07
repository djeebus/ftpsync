package lib

import (
	"path/filepath"

	"github.com/pkg/errors"
)

type ListResult struct {
	Files   map[string]int64
	Folders []string
}

func NewListResult() ListResult {
	return ListResult{
		Files: make(map[string]int64),
	}
}

type Lister interface {
	List(path string) (ListResult, error)
}

func WalkLister(lister Lister, rootPath string) (*SizeSet, error) {
	result := NewSizeSet()

	work := Queue[string]{MaxSize: 1000}
	work.Enqueue(rootPath)

	for !work.IsEmpty() {
		path := work.Dequeue()

		results, err := lister.List(path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read files")
		}

		for _, d := range results.Folders {
			fullpath := filepath.Join(path, d)
			work.Enqueue(fullpath)
		}

		for filename, size := range results.Files {
			fullPath := filepath.Join(path, filename)
			result.Set(fullPath, size)
		}
	}

	return result, nil
}
