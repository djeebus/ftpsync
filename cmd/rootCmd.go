package cmd

import (
	"net/url"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"sync"

	"github.com/djeebus/ftpsync/lib/deluge"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/djeebus/ftpsync/lib"
	"github.com/djeebus/ftpsync/lib/filebrowser"
	"github.com/djeebus/ftpsync/lib/ftp"
	"github.com/djeebus/ftpsync/lib/localfs"
	"github.com/djeebus/ftpsync/lib/sqlite"
)

var allNumbers = regexp.MustCompile(`^\d+$`)

func parseFsMode(mode string) (os.FileMode, error) {
	mode64, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse mode")
	}
	return os.FileMode(mode64), nil
}

type lookupName func(name string) (string, error)

func parseId(id string, lookupName lookupName) (int, error) {
	var err error

	if !allNumbers.MatchString(id) {
		id, err = lookupName(id)
	}

	if err != nil {
		return 0, errors.Wrap(err, "failed to lookup id")
	}

	return strconv.Atoi(id)
}

var (
	currentUserOnce sync.Once
	currentUser     *user.User
)

func getCurrentUser() *user.User {
	currentUserOnce.Do(func() {
		u, err := user.Current()
		if err != nil {
			panic(err)
		}
		currentUser = u

	})

	return currentUser
}

func parseUser(userID string) (int, error) {
	if userID == "" {
		userID = getCurrentUser().Uid
	}

	return parseId(userID, func(name string) (string, error) {
		if u, err := user.Lookup(name); err != nil {
			return "", errors.Wrapf(err, "failed to lookup '%s' user", name)
		} else {
			return u.Uid, nil
		}
	})
}

func parseGroup(groupId string) (int, error) {
	if groupId == "" {
		groupId = getCurrentUser().Gid
	}

	return parseId(groupId, func(name string) (string, error) {
		if g, err := user.LookupGroup(name); err != nil {
			return "", errors.Wrapf(err, "failed to lookup '%s' group", name)
		} else {
			return g.Gid, nil
		}

	})
}

func parseLogLevel(logLevelStr string) (logrus.Level, error) {
	return logrus.ParseLevel(logLevelStr)
}

func doSync(src, pre, dst string, log logrus.FieldLogger) error {
	var (
		err         error
		precheckURL *url.URL
		srcURL      *url.URL
		source      lib.Source
		precheck    lib.Precheck
		database    lib.Database
		destination lib.Destination

		dirMode  os.FileMode
		fileMode os.FileMode

		fileUserID  int
		fileGroupID int
	)

	if pre != "" {
		precheckURL, err = url.Parse(pre)
		if err != nil {
			return errors.Wrap(err, "failed to parse precheck")
		}
	}

	srcURL, err = url.Parse(src)
	if err != nil {
		return errors.Wrap(err, "fail to parse url")
	}

	if fileMode, err = parseFsMode(fileModeStr); err != nil {
		return errors.Wrap(err, "failed to parse file mode")
	}
	if dirMode, err = parseFsMode(dirModeStr); err != nil {
		return errors.Wrap(err, "failed to parse dir mode")
	}
	if fileUserID, err = parseUser(fileUserStr); err != nil {
		return errors.Wrap(err, "failed to parse file mode")
	}
	if fileGroupID, err = parseGroup(fileGroupStr); err != nil {
		return errors.Wrap(err, "failed to parse group id")
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
	default:
		return errors.New("unknown source")
	}

	switch precheckURL.Scheme {
	case "deluge", "deluges":
		if precheck, err = deluge.New(log, precheckURL, rootDir); err != nil {
			if err != nil {
				return errors.Wrap(err, "failed to create deluge precheck")
			}
		}
	}

	if database, err = sqlite.New(dbLocation); err != nil {
		return errors.Wrap(err, "failed to build database")
	}
	if destination, err = localfs.New(dst, dirMode, fileMode, fileUserID, fileGroupID); err != nil {
		return errors.Wrap(err, "failed to build destination")
	}

	processor := lib.BuildProcessor(source, database, precheck, destination, log)
	return processor.Process(rootDir)
}
