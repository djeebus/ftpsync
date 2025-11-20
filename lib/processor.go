package lib

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func BuildProcessor(src Source, db Database, precheck Precheck, dst Destination, log logrus.FieldLogger) *Processor {
	return &Processor{src, db, dst, precheck, log}
}

type Processor struct {
	remote   Source
	db       Database
	local    Destination
	precheck Precheck
	log      logrus.FieldLogger
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

	if err = p.local.CleanDirectories(rootPath); err != nil {
		return errors.Wrap(err, "failed to clean directories")
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

	log.Info("downloading")

	fp, err := p.remote.Read(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", path)
	}

	start := time.Now()
	bytes, err := p.local.Write(path, fp)
	if err != nil {
		return errors.Wrapf(err, "failed to write %s", path)
	}
	defer func() {
		if err := fp.Close(); err != nil {
			fmt.Printf("failed to close reader for %s: %v", path, err)
		}
	}()

	done := time.Since(start)
	log.WithFields(logrus.Fields{
		"bytes_str": fmtSize(bytes),
		"bytes":     bytes,
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
