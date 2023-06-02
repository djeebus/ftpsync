package lib

import (
	"path/filepath"

	"github.com/pkg/errors"
)

type ListResult struct {
	Files   []string
	Folders []string
}

type Lister interface {
	List(path string) (ListResult, error)
}

func WalkLister(lister Lister, rootPath string) (*Set, error) {
	result := NewSet()

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

		for _, f := range results.Files {
			fullPath := filepath.Join(path, f)
			result.Set(fullPath)
		}
	}

	return result, nil
}
