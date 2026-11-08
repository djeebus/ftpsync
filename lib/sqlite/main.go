package sqlite

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	"github.com/djeebus/ftpsync/lib"
)

const createFilesTable = `
CREATE TABLE IF NOT EXISTS files (
    path 	TEXT 		NOT NULL 	PRIMARY KEY,
    time 	DATETIME 	NOT NULL	DEFAULT CURRENT_TIMESTAMP
)
`

type database struct {
	db *sql.DB
}

var _ lib.Database = new(database)

func New(dbPath string) (lib.Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", dbPath)
	}

	if _, err = db.Exec(createFilesTable); err != nil {
		return nil, errors.Wrap(err, "failed to create files table")
	}

	return &database{db}, nil
}

func (s *database) GetAllFiles(rootPath string) (*lib.Set, error) {
	// compare the prefix directly, LIKE is case-insensitive and treats _ and % as wildcards
	prefix := strings.TrimRight(rootPath, "/") + "/"
	rows, err := s.db.Query(
		`SELECT path FROM files WHERE path = ? OR substr(path, 1, length(?)) = ?`,
		rootPath, prefix, prefix,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all files")
	}
	defer rows.Close()

	var path string

	files := lib.NewSet()
	for rows.Next() {
		if err = rows.Scan(&path); err != nil {
			return nil, errors.Wrap(err, "failed to scan")
		}

		files.Set(path)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to read files")
	}

	return files, nil
}

func (s *database) Exists(path string) (bool, error) {
	var c int

	row := s.db.QueryRow(`SELECT 1 FROM files WHERE path = ?`, path)
	err := row.Scan(&c)

	switch err {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, errors.Wrapf(err, "failed to query for %s", path)
	}
}

func (s *database) Record(path string) error {
	if _, err := s.db.Exec(`
INSERT INTO files (path) VALUES (?)
ON CONFLICT (path) DO UPDATE SET time = CURRENT_TIMESTAMP
`, path); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

func (s *database) Delete(path string) error {
	if _, err := s.db.Exec(
		`DELETE FROM files WHERE path = ?`,
		path,
	); err != nil {
		return errors.Wrap(err, "failed to delete a file")
	}

	return nil
}

func (s *database) Close() error {
	return s.db.Close()
}
