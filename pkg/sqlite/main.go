package sqlite

import (
	"database/sql"

	"github.com/djeebus/ftpsync/pkg"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

const fileLocation = "ftpsync.db"

const createFilesTable = `
CREATE TABLE IF NOT EXISTS files (
    path 	STRING 		NOT NULL 	PRIMARY KEY,
    time 	DATETIME 	NOT NULL	DEFAULT CURRENT_TIMESTAMP,
    jobID 	STRING 		NOT NULL
)
`

func BuildDatabase() (pkg.Database, error) {
	db, err := sql.Open("sqlite3", fileLocation)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", fileLocation)
	}

	if _, err = db.Exec(createFilesTable); err != nil {
		return nil, errors.Wrap(err, "failed to create files table")
	}

	return &SqliteDb{db}, nil
}

type SqliteDb struct {
	db *sql.DB
}

func (s *SqliteDb) Exists(path string) (bool, error) {
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

func (s *SqliteDb) Record(path, jobID string) error {
	if _, err := s.db.Exec(
		"INSERT INTO files (path, jobID) VALUES (?, ?)",
		path, jobID); err != nil {
		return errors.Wrapf(err, "failed to record %s", path)
	}

	return nil
}

var _ pkg.Database = new(SqliteDb)
