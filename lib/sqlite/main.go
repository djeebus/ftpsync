package sqlite

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	"github.com/djeebus/ftpsync/lib"
)

const createFilesTable = `
CREATE TABLE IF NOT EXISTS files (
    path 	STRING 		NOT NULL 	PRIMARY KEY,
    time 	DATETIME 	NOT NULL	DEFAULT CURRENT_TIMESTAMP,
    jobID 	STRING 		NOT NULL
)
`

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

type database struct {
	db *sql.DB
}

func (s *database) GetAllFiles(rootPath string) (*lib.Set, error) {
	row, err := s.db.Query(`SELECT path FROM files WHERE path LIKE '?%'`, rootPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all files")
	}

	var path string

	files := lib.NewSet()
	for row.Next() {
		if err = row.Scan(&path); err != nil {
			return nil, errors.Wrap(err, "failed to scan")
		}

		files.Set(path)
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
	if _, err := s.db.Exec(
		"INSERT INTO files (path, jobID) VALUES (?, ?)",
		path, ""); err != nil {
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

var _ lib.Database = new(database)
