package lib

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func BuildProcessor(src Source, db Database, precheck Precheck, dst Destination, log logrus.FieldLogger) *Processor {
	return &Processor{remote: src, db: db, local: dst, precheck: precheck, log: log, remoteFiles: NewSizeSet()}
}

type Processor struct {
	remote   Source
	db       Database
	local    Destination
	precheck Precheck
	log      logrus.FieldLogger

	// remoteFiles is populated by Process
	remoteFiles *SizeSet
}

type FileStatusKey struct {
	HasRemote, IsRecorded, HasLocal bool
}

func (key FileStatusKey) String() string {
	return fmt.Sprintf(
		"remote:%s, record:%s, local:%s",
		strconv.FormatBool(key.HasRemote),
		strconv.FormatBool(key.IsRecorded),
		strconv.FormatBool(key.HasLocal),
	)
}

func (key FileStatusKey) InSync() bool {
	return key.HasRemote && key.IsRecorded && key.HasLocal
}

type TruthAction func(FileStatusKey, *Processor, string) error
type NamedAction struct {
	Action TruthAction
	Name   string
}

var fileStatusActions = map[FileStatusKey]NamedAction{
	{true, false, false}:  {downloadFile, "download"},
	{true, true, false}:   {downloadFile, "download"},
	{true, true, true}:    {skipFile, "skip"},
	{false, true, false}:  {deleteFile, "delete"},
	{false, true, true}:   {deleteFile, "delete"},
	{false, false, true}:  {deleteFile, "delete"},
	{false, false, false}: {logFile, "log"},
	{true, false, true}:   {recordFile, "record"},
}

func (p *Processor) Process(rootPath string) error {
	var (
		err error

		remoteFiles *SizeSet
		dbFiles     *Set
		localFiles  *SizeSet
	)

	if remoteFiles, err = p.remote.GetAllFiles(rootPath); err != nil {
		return errors.Wrap(err, "failed to get ftp files")
	}
	p.remoteFiles = remoteFiles
	p.log.WithField("count", remoteFiles.Len()).Info("found remote files")

	if dbFiles, err = p.db.GetAllFiles(rootPath); err != nil {
		return errors.Wrap(err, "failed to get database files")
	}
	p.log.WithField("count", dbFiles.Len()).Info("found recorded files")

	if localFiles, err = p.local.GetAllFiles(rootPath); err != nil {
		return errors.Wrap(err, "failed to get local files")
	}
	p.log.WithField("count", localFiles.Len()).Info("found local files")

	allFiles := NewSet().Union(remoteFiles.ToSet()).Union(dbFiles).Union(localFiles.ToSet())
	p.log.WithField("count", allFiles.Len()).Info("total files found")

	for _, file := range allFiles.ToList() {
		log := p.log.WithField("file", file)
		hasDbFile := dbFiles.Has(file)
		localSize, hasLocalFile := localFiles.Get(file)
		remoteSize, hasRemoteFile := remoteFiles.Get(file)
		if hasLocalFile && hasRemoteFile && localSize != remoteSize {
			log.Warning("local file out of sync from remote file, deleting")
			if err := p.local.Delete(file); err != nil {
				log.WithError(err).Error("failed to delete local file")
				continue
			}
			hasLocalFile = false
		}

		key := FileStatusKey{
			IsRecorded: hasDbFile,
			HasRemote:  hasRemoteFile,
			HasLocal:   hasLocalFile,
		}
		action := fileStatusActions[key]

		if !(key.InSync()) && action.Name != "skip" {
			log.
				WithField("action", action.Name).
				WithField("state", key.String()).
				Info("out of sync")
		}

		if err = action.Action(key, p, file); err != nil {
			log.
				WithField("action", action.Name).
				WithField("state", key.String()).
				WithError(err).
				Error("action failed")
			continue
		}
	}

	if err = p.cleanPartials(rootPath); err != nil {
		return errors.Wrap(err, "failed to clean partial downloads")
	}

	if err = p.local.CleanDirectories(rootPath); err != nil {
		return errors.Wrap(err, "failed to clean directories")
	}

	return nil
}

// cleanPartials deletes partial downloads that are no longer needed, either
// because the download completed or the file is gone from the remote.
func (p *Processor) cleanPartials(rootPath string) error {
	partials, err := p.local.GetAllPartials(rootPath)
	if err != nil {
		return err
	}

	for _, path := range partials.ToList() {
		log := p.log.WithField("path", path)

		isComplete, err := p.local.Exists(path)
		if err != nil {
			log.WithError(err).Error("failed to check for completed file")
			continue
		}

		if p.remoteFiles.Has(path) && !isComplete {
			continue
		}

		log.WithField("complete", isComplete).Info("deleting partial download")
		if err = p.local.DeletePartial(path); err != nil {
			log.WithError(err).Error("failed to delete partial download")
		}
	}

	return nil
}

func downloadFile(_ FileStatusKey, p *Processor, path string) error {
	log := p.log.WithField("path", path)

	if p.precheck != nil {
		log.Info("checking to see if file should be downloaded")
		ok, err := p.precheck.IsFileReady(path)
		if err != nil {
			return errors.Wrap(err, "failed to precheck file")
		}

		if !ok {
			log.Info("skipping file, not yet ready")
			return nil
		}

		log.Info("file is ready for download")
	}

	offset, err := p.local.PartialSize(path)
	if err != nil {
		return errors.Wrapf(err, "failed to check partial download of %s", path)
	}
	if remoteSize, ok := p.remoteFiles.Get(path); ok && offset >= remoteSize {
		// partial is complete or bogus, either way start over
		offset = 0
	}

	log.WithField("offset", offset).Info("downloading")

	fp, offset, err := p.remote.ReadAt(path, offset)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", path)
	}
	defer func() {
		if err := fp.Close(); err != nil {
			log.WithError(err).Warning("failed to close reader")
		}
	}()

	start := time.Now()
	size, err := p.local.Write(path, fp, offset)
	bytes := size - offset
	if err != nil {
		return fmt.Errorf("failed to write %s (wrote %d bytes, %d total): %w", path, bytes, size, err)
	}

	done := time.Since(start)
	log.WithFields(logrus.Fields{
		"bytes_str": fmtSize(bytes),
		"bytes":     bytes,
		"resumed":   offset,
		"seconds":   int64(done.Seconds()),
		"speed":     fmtSpeed(bytes, done),
	}).Info("download complete")
	if err = p.db.Record(path); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

func recordFile(_ FileStatusKey, p *Processor, path string) error {
	log := p.log.WithField("path", path)
	log.Info("recording")

	if err := p.db.Record(path); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

func skipFile(_ FileStatusKey, _ *Processor, _ string) error {
	return nil
}

func deleteFile(key FileStatusKey, p *Processor, path string) error {
	log := p.log.WithField("path", path)

	if key.IsRecorded {
		log.Info("deleting record")
		if err := p.db.Delete(path); err != nil {
			return errors.Wrap(err, "error unrecording file")
		}
	}

	if key.HasLocal {
		log.Info("deleting local file")
		if err := p.local.Delete(path); err != nil {
			return errors.Wrap(err, "error deleting file")
		}
	}

	return nil
}

func logFile(key FileStatusKey, p *Processor, path string) error {
	log := p.log.WithField("path", path)
	log.WithField("state", key.String()).Warn("file is in a weird state")
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
