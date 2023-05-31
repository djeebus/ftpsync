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

func (p *Processor) Process(rootPath string) error {
	jobID := uuid.NewString()
	work := Queue[string]{MaxSize: 1000}
	work.Enqueue(rootPath)

	for !work.IsEmpty() {
		path := work.Dequeue()

		fmt.Println(fmt.Sprintf(">> %s", path))

		results, err := p.src.List(path)
		if err != nil {
			return errors.Wrap(err, "failed to read files")
		}

		for _, d := range results.Folders {
			fullpath := filepath.Join(path, d)
			work.Enqueue(fullpath)
		}

		// ftp=yes
		for _, f := range results.Files {
			fullPath := filepath.Join(path, f)

			if ok, err := p.db.Exists(fullPath); err != nil {
				return errors.Wrapf(err, "failed to check db for path %s", fullPath)
			} else if ok {
				// ftp=yes,db=yes,local=no
				// ftp=yes,db=yes,local=yes
				continue
			}

			if ok, err := p.dst.Exists(fullPath); err != nil {
				return errors.Wrapf(err, "failed to check local for path %s", fullPath)
			} else if ok {
				// ftp=yes,db=no,local=yes
				continue
			}

			// ftp=yes,db=no,local=no
			fmt.Printf("+++ downloading %s ... ", fullPath)
			fp, err := p.src.Read(fullPath)
			if err != nil {
				return errors.Wrapf(err, "failed to read %s", fullPath)
			}

			start := time.Now()
			bytes, err := p.dst.Write(fullPath, fp)
			if err != nil {
				return errors.Wrapf(err, "failed to write %s", fullPath)
			}
			done := time.Since(start)
			fmt.Printf("%s in %d seconds, %s\n", fmtSize(bytes), int64(done.Seconds()), fmtSpeed(bytes, done))
			if err = p.db.Record(fullPath, jobID); err != nil {
				return errors.Wrapf(err, "failed to record %s", fullPath)
			}
		}
	}

	return nil
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

// truth matrix
// ftp	db		local	action
// yes	no		no		download

// yes	yes		no		skip
// yes	yes		yes		skip

// no	yes		no		delete
// no	yes		yes		delete

// no	no		yes		log
// no	no		no		log
// yes	no		yes		log
