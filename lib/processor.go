package lib

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

func BuildProcessor(src Source, db Database, dst Destination) *Processor {
	return &Processor{src, db, dst}
}

type Processor struct {
	remote Source
	db     Database
	local  Destination
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
	{true, true, false}:   {skipFile, "skip"},
	{true, true, true}:    {skipFile, "skip"},
	{false, true, false}:  {deleteFile, "delete"},
	{false, true, true}:   {deleteFile, "delete"},
	{false, false, true}:  {deleteFile, "delete"},
	{false, false, false}: {logFile, "log"},
	{true, false, true}:   {recordFile, "record"},
}

func echo(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Println(msg)
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
	echo("found %d remote files", remoteFiles.Len())

	if dbFiles, err = p.db.GetAllFiles(rootPath); err != nil {
		return errors.Wrap(err, "failed to get database files")
	}
	echo("found %d recorded files", dbFiles.Len())

	if localFiles, err = p.local.GetAllFiles(rootPath); err != nil {
		return errors.Wrap(err, "failed to get local files")
	}
	echo("found %d local files", localFiles.Len())

	allFiles := NewSet().Union(remoteFiles.ToSet()).Union(dbFiles).Union(localFiles.ToSet())
	echo("found a combined total of %d files", allFiles.Len())

	for _, file := range allFiles.ToList() {
		hasDbFile := dbFiles.Has(file)
		localSize, hasLocalFile := localFiles.Get(file)
		remoteSize, hasRemoteFile := remoteFiles.Get(file)
		if hasLocalFile && hasRemoteFile && localSize != remoteSize {
			echo("local file out of sync from remote file, deleting")
			if err := p.local.Delete(file); err != nil {
				echo("failed to delete local file: %v", err)
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

		if !(key.InSync()) {
			echo("%s out of sync (%s), action = %s",
				file,
				key.String(),
				action.Name,
			)
		}

		if err = action.Action(key, p, file); err != nil {
			return errors.Wrapf(err, "failed to perform action for %s", file)
		}
	}

	if err = p.local.CleanDirectories(rootPath); err != nil {
		return errors.Wrap(err, "failed to clean directories")
	}

	return nil
}

func downloadFile(_ FileStatusKey, p *Processor, path string) error {
	echo("+++ downloading %s ... ", path)
	fp, err := p.remote.Read(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", path)
	}

	start := time.Now()
	bytes, err := p.local.Write(path, fp)
	if err != nil {
		return errors.Wrapf(err, "failed to write %s", path)
	}
	done := time.Since(start)
	echo("%s in %d seconds, %s", fmtSize(bytes), int64(done.Seconds()), fmtSpeed(bytes, done))
	if err = p.db.Record(path); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

func recordFile(_ FileStatusKey, p *Processor, path string) error {
	echo("@@@ recording %s", path)
	if err := p.db.Record(path); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

func skipFile(_ FileStatusKey, _ *Processor, _ string) error {
	return nil
}

func deleteFile(key FileStatusKey, p *Processor, path string) error {
	if key.IsRecorded {
		echo("@@@ deleting %s from the database", path)
		if err := p.db.Delete(path); err != nil {
			return errors.Wrap(err, "error unrecording file")
		}
	}

	if key.HasLocal {
		echo("--- deleting %s from the file system", path)
		if err := p.local.Delete(path); err != nil {
			return errors.Wrap(err, "error deleting file")
		}
	}

	return nil
}

func logFile(key FileStatusKey, _ *Processor, path string) error {
	echo("!!! file %s is in a weird state: [ftp: %t, db: %t, local: %t],  !!!", path, key.HasRemote, key.IsRecorded, key.HasLocal)
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
