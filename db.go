package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS dump (
  id INTEGER PRIMARY KEY ASC AUTOINCREMENT,
  path TEXT DEFAULT "" NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS content USING fts4 (
  text DEFAULT "",
  comment DEFAULT ""
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
	db  *sql.DB
	ctx context.Context
}

type Content struct {
	Id   int
	Text string
	Date time.Time
}

func CreateDb(path string, ctx context.Context) (*Db, error) {
	dbfile := filepath.Join(path, "jsondump.sqlite3")
	d, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", dbfile))
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
		db:  d,
		ctx: ctx,
	}

	return ret, nil
}

func (db *Db) Add(path, content string) error {
	return nil
}

func (db *Db) Delete(path, id string) error {
	return nil
}

func (db *Db) Get(path, id string) ([]Content, error) {
	return nil, nil
}

func (db *Db) Close() error {
	return db.db.Close()
}
