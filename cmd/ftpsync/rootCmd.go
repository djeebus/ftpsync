package main

import (
	"net/url"
	"os"

	"github.com/djeebus/ftpsync/pkg"
	"github.com/djeebus/ftpsync/pkg/filebrowser"
	"github.com/djeebus/ftpsync/pkg/ftp"
	"github.com/djeebus/ftpsync/pkg/localfs"
	"github.com/djeebus/ftpsync/pkg/sqlite"
	"github.com/pkg/errors"
)

func doSync(src, dst string) error {

	var (
		err         error
		srcURL      *url.URL
		source      pkg.Source
		database    pkg.Database
		destination pkg.Destination

		dirMode  os.FileMode
		fileMode os.FileMode
	)

	srcURL, err = srcURL.Parse(src)
	if err != nil {
		return errors.Wrap(err, "fail to parse url")
	}

	if fileMode, err = parseFsMode(fileModeStr); err != nil {
		return errors.Wrap(err, "failed to parse file mode")
	}
	if dirMode, err = parseFsMode(dirModeStr); err != nil {
		return errors.Wrap(err, "failed to parse dir mode")
	}

	switch srcURL.Scheme {
	case "ftp", "ftps", "sftp":
		if source, err = ftp.BuildSource(srcURL); err != nil {
			return errors.Wrap(err, "failed to build ftp source")
		}
	case "filebrowser":
		if source, err = filebrowser.BuildSource(srcURL); err != nil {
			return errors.Wrap(err, "failed to build filebrowser source")
		}
	}

	if database, err = sqlite.BuildDatabase(dbLocation); err != nil {
		return errors.Wrap(err, "failed to build database")
	}
	if destination, err = localfs.BuildDestination(dst, dirMode, fileMode); err != nil {
		return errors.Wrap(err, "failed to build destination")
	}

	processor := pkg.BuildProcessor(source, database, destination)
	return processor.Process(rootDir)
}
