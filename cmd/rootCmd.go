package cmd

import (
	"net/url"
	"os"

	"github.com/pkg/errors"

	"github.com/djeebus/ftpsync/lib"
	"github.com/djeebus/ftpsync/lib/filebrowser"
	"github.com/djeebus/ftpsync/lib/ftp"
	"github.com/djeebus/ftpsync/lib/localfs"
	"github.com/djeebus/ftpsync/lib/sqlite"
)

func doSync(src, dst string) error {

	var (
		err         error
		srcURL      *url.URL
		source      lib.Source
		database    lib.Database
		destination lib.Destination

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
		if source, err = ftp.New(srcURL); err != nil {
			return errors.Wrap(err, "failed to build ftp source")
		}
	case "filebrowser":
		if source, err = filebrowser.New(srcURL); err != nil {
			return errors.Wrap(err, "failed to build filebrowser source")
		}
	}

	if database, err = sqlite.New(dbLocation); err != nil {
		return errors.Wrap(err, "failed to build database")
	}
	if destination, err = localfs.New(dst, dirMode, fileMode); err != nil {
		return errors.Wrap(err, "failed to build destination")
	}

	processor := lib.BuildProcessor(source, database, destination)
	return processor.Process(rootDir)
}
