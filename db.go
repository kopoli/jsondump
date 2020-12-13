package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const defaultVersions = 10

const schema = `
CREATE TABLE IF NOT EXISTS dump (
  id INTEGER PRIMARY KEY ASC AUTOINCREMENT,
  path TEXT DEFAULT "" NOT NULL UNIQUE ON CONFLICT ABORT
);

CREATE VIRTUAL TABLE IF NOT EXISTS content USING fts4 (
  text DEFAULT "",
);

CREATE TABLE IF NOT EXISTS dumpcontent (
  dumpid INTEGER REFERENCES dump(id) NOT NULL,
  contentid INTEGER REFERENCES content(rowid) NOT NULL,
  added DATETIME,
  UNIQUE (dumpid, contentid)
);

PRAGMA busy_timeout=10000;
PRAGMA user_version=1;
`

type Db struct {
	db          *sql.DB
	ctx         context.Context
	MaxVersions int
}

type Content struct {
	Id   int
	Text string
	Date time.Time
}

func CreateDb(path string, ctx context.Context) (*Db, error) {
	// dbfile := filepath.Join(path, "jsondump.sqlite3")
	d, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", path))
	if err != nil {
		return nil, err
	}

	_, err = d.ExecContext(ctx, schema)
	if err != nil {
		_ = d.Close()
		return nil, err
	}

	d.SetMaxOpenConns(1)

	ret := &Db{
		db:          d,
		ctx:         ctx,
		MaxVersions: defaultVersions,
	}

	return ret, nil
}

func (db *Db) Add(path, content string) error {
	pathquery := `
-- Possibly insert a new path to the DB
INSERT INTO dump(path)
SELECT $1
WHERE NOT EXISTS (SELECT 1 FROM dump WHERE path = $1);

-- Remove excess elements from the path

-- Insert new content
`

	added := time.Now()

	// db.db.QueryContext(ctx context.Context, query string, args ...interface{})
	_, err := db.db.ExecContext(db.ctx, pathquery,
		path, db.MaxVersions, content, added)
	if err != nil {
		return err
	}

	return nil
}

func (db *Db) Delete(path, id string) error {
	return nil
}

func (db *Db) GetPaths() ([]string, error) {
	return nil, nil
}

func (db *Db) GetContent(path, id string) ([]Content, error) {
	return nil, nil
}

func (db *Db) Close() error {
	return db.db.Close()
}
