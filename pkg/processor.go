package pkg

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func BuildProcessor(src Source, db Database, dst Destination) *Processor {
	return &Processor{src, db, dst}
}

type Processor struct {
	src Source
	db  Database
	dst Destination
}

type FileStatusKey struct {
	HasFtp, HasDb, HasLocal bool
}

type TruthAction func(FileStatusKey, *Processor, string, string) error

var fileStatusActions = map[FileStatusKey]TruthAction{
	{true, false, false}:  downloadFile,
	{true, true, false}:   skipFile,
	{true, true, true}:    skipFile,
	{false, true, false}:  deleteFile,
	{false, true, true}:   deleteFile,
	{false, false, true}:  deleteFile,
	{false, false, false}: logFile,
	{true, false, true}:   recordFile,
}

func (p *Processor) Process(rootPath string) error {
	var (
		err error

		ftpFiles   *Set
		dbFiles    *Set
		localFiles *Set
	)

	jobID := uuid.NewString()

	if ftpFiles, err = p.getFtpFiles(rootPath); err != nil {
		return errors.Wrap(err, "failed to get ftp files")
	}

	if dbFiles, err = p.getDbFiles(); err != nil {
		return errors.Wrap(err, "failed to get database files")
	}

	if localFiles, err = p.getLocalFiles(rootPath); err != nil {
		return errors.Wrap(err, "failed to get local files")
	}

	allFiles := NewSet().Union(ftpFiles).Union(dbFiles).Union(localFiles)

	for _, file := range allFiles.ToList() {
		hasFtpFile := ftpFiles.Has(file)
		hasDbFile := dbFiles.Has(file)
		hasLocalFile := localFiles.Has(file)

		key := FileStatusKey{
			HasDb:    hasDbFile,
			HasFtp:   hasFtpFile,
			HasLocal: hasLocalFile,
		}
		action := fileStatusActions[key]
		if err = action(key, p, file, jobID); err != nil {
			return errors.Wrapf(err, "failed to perform action for %s", file)
		}
	}

	return nil
}

func downloadFile(key FileStatusKey, p *Processor, path, jobID string) error {
	fmt.Printf("+++ downloading %s ... ", path)
	fp, err := p.src.Read(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", path)
	}

	start := time.Now()
	bytes, err := p.dst.Write(path, fp)
	if err != nil {
		return errors.Wrapf(err, "failed to write %s", path)
	}
	done := time.Since(start)
	fmt.Printf("%s in %d seconds, %s\n", fmtSize(bytes), int64(done.Seconds()), fmtSpeed(bytes, done))
	if err = p.db.Record(path, jobID); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

func recordFile(key FileStatusKey, p *Processor, path, jobID string) error {
	if err := p.db.Record(path, jobID); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

func skipFile(key FileStatusKey, p *Processor, path, jobID string) error {
	return nil
}

func deleteFile(key FileStatusKey, p *Processor, path, jobID string) error {
	if key.HasDb {
		if err := p.db.Delete(path); err != nil {
			return errors.Wrap(err, "error unrecording file")
		}
	}

	if key.HasLocal {
		if err := p.dst.Delete(path); err != nil {
			return errors.Wrap(err, "error deleting file")
		}
	}

	return nil
}

func logFile(key FileStatusKey, p *Processor, path, jobID string) error {
	fmt.Printf("!!! file %s is in a weird state: [ftp: %t, db: %t, local: %t],  !!!\n", path, key.HasFtp, key.HasDb, key.HasLocal)
	return nil
}

func (p *Processor) getFtpFiles(rootPath string) (*Set, error) {
	ftpFiles := NewSet()

	work := Queue[string]{MaxSize: 1000}
	work.Enqueue(rootPath)

	for !work.IsEmpty() {
		path := work.Dequeue()

		fmt.Println(fmt.Sprintf(">> %s", path))

		results, err := p.src.List(path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read files")
		}

		for _, d := range results.Folders {
			fullpath := filepath.Join(path, d)
			work.Enqueue(fullpath)
		}

		// ftp=yes
		for _, f := range results.Files {
			fullPath := filepath.Join(path, f)
			ftpFiles.Set(fullPath)
		}
	}

	return ftpFiles, nil
}

func (p *Processor) getDbFiles() (*Set, error) {
	files, err := p.db.GetAllFiles()
	if err != nil {
		return nil, errors.Wrap(err, "error getting database files")
	}

	return files, nil
}

func (p *Processor) getLocalFiles(rootPath string) (*Set, error) {
	files, err := p.dst.GetAllFiles(rootPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get destination files")
	}

	return files, nil
}

var markers = []string{"B", "KB", "MB", "GB", "TB"}

func fmtSpeed(bytes int64, done time.Duration) string {
	speed := float64(bytes) / done.Seconds()
	idx := 0
	for speed > 1024 && idx < len(markers) {
		idx++
		speed /= 1024
	}

	return fmt.Sprintf("%.2f%s/s", speed, markers[idx])
}

func fmtSize(bytes int64) string {
	b := float64(bytes)
	idx := 0
	for b > 1024 {
		idx++
		b /= 1024
	}

	return fmt.Sprintf("%.2f%s", b, markers[idx])
}
