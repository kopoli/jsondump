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
  contentid INTEGER REFERENCES content(rowid) ON DELETE CASCADE NOT NULL,
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

func (db *Db) exec(queries []string, args ...interface{}) error {
	tx, err := db.db.BeginTx(db.ctx, nil)
	if err != nil {
		return err
	}

	for _, query := range queries {
		_, err = tx.ExecContext(db.ctx, query,
			args...,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *Db) Add(path, content string) error {
	queries := []string{
		`-- Possibly insert a new path to the DB
INSERT INTO dump(path)
SELECT @path
WHERE NOT EXISTS (SELECT 1 FROM dump WHERE path = @path);`,

		`-- Remove excess elements from the path
DELETE FROM content WHERE content.rowid = (
  SELECT contentid FROM dumpcontent WHERE dumpcontent.added <= (
    SELECT MAX(added) FROM (
      SELECT added FROM dumpcontent ORDER BY added LIMIT 0, @max)));`,

		`-- Insert new content
INSERT INTO content(text) VALUES (@content);`,

		`-- Insert new bindings
INSERT INTO dumpcontent(dumpid, contentid, added)
SELECT dump.id, content.rowid, @added FROM dump, content
WHERE dump.path = @path AND content.text = @content;
`}

	added := time.Now()

	return db.exec(queries,
		sql.Named("path", path),
		sql.Named("content", content),
		sql.Named("added", added),
		sql.Named("max", db.MaxVersions),
	)
}

func (db *Db) Delete(path, id string) error {
	return nil
}

func (db *Db) GetPaths() ([]string, error) {
	query := `
SELECT path FROM dump ORDER BY path ASC;
`
	rows, err := db.db.QueryContext(db.ctx, query)
	if err != nil {
		return nil, err
	}

	ret := []string{}
	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			return nil, err
		}
		ret = append(ret, path)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (db *Db) GetContent(path, id string) ([]Content, error) {
	return nil, nil
}

func (db *Db) Close() error {
	return db.db.Close()
}
