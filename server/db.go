package jsondump

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	defaultVersions = 10
	replaceInterval = time.Hour * 24
)

const schema = `
CREATE TABLE IF NOT EXISTS dump (
  id INTEGER PRIMARY KEY ASC AUTOINCREMENT,
  path TEXT DEFAULT "" NOT NULL UNIQUE ON CONFLICT ABORT
);

CREATE TABLE IF NOT EXISTS content (
  id INTEGER PRIMARY KEY ASC AUTOINCREMENT,
  text DEFAULT "" NOT NULL,
  added DATETIME,
  dumpid INTEGER REFERENCES dump(id) NOT NULL
);

PRAGMA busy_timeout=10000;
PRAGMA user_version=1;
`

type Db struct {
	db          *sql.DB
	ctx         context.Context
	MaxVersions int
	ReplaceInterval time.Duration
}

type Content struct {
	Id   int
	Text string
	Date time.Time
}

func CreateDb(path string, ctx context.Context) (*Db, error) {
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
		ReplaceInterval: replaceInterval,
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

		`-- Delete content that is in the replace interval
DELETE FROM content
WHERE content.dumpid = (SELECT id FROM dump WHERE path = @path) AND
  strftime('%s', content.added) > strftime('%s', @from) AND
  strftime('%s', content.added) <= strftime('%s', @added);
`,
		`-- Insert new content
INSERT INTO content(text, added, dumpid)
SELECT @content AS text, @added AS added, dump.id
FROM dump
WHERE dump.path = @path;
`,
		`-- Remove excess elements from the content table
DELETE FROM content
WHERE content.dumpid = (SELECT id FROM dump WHERE path = @path) AND
  content.id IN (SELECT id FROM content ORDER BY id DESC LIMIT -1 OFFSET @max);
`,
	}

	added := time.Now()
	replaceTime := added.Add(-db.ReplaceInterval)

	return db.exec(queries,
		sql.Named("path", path),
		sql.Named("content", content),
		sql.Named("added", added),
		sql.Named("from", replaceTime),
		sql.Named("max", db.MaxVersions),
	)
}

func (db *Db) Delete(path string) error {
	queries := []string{
		`-- Remove excess elements from the junction table
DELETE FROM content
WHERE content.dumpid = (SELECT id FROM dump WHERE path = @path);
`,
		`-- Delete path
DELETE FROM dump
WHERE dump.path = @path;
`}
	return db.exec(queries,
		sql.Named("path", path),
	)
}

func (db *Db) query(query string, handleRow func(*sql.Rows) error,
	args ...interface{}) error {

	rows, err := db.db.QueryContext(db.ctx, query, args...)
	if err != nil {
		return err
	}

	for rows.Next() {
		err = handleRow(rows)
		if err != nil {
			return err
		}
	}
	return rows.Err()
}

func (db *Db) GetPaths() ([]string, error) {
	query := `
SELECT path FROM dump ORDER BY path ASC;
`
	ret := []string{}
	row := func(rows *sql.Rows) error {
		var path string
		err := rows.Scan(&path)
		if err != nil {
			return err
		}
		ret = append(ret, path)
		return nil
	}

	err := db.query(query, row)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (db *Db) GetContent(path string, numLatest int) ([]Content, error) {
	query := `
SELECT content.id, content.text, content.added
FROM content, dump
WHERE dump.path = @path
ORDER BY content.added DESC
LIMIT @limit;
`
	ret := []Content{}

	row := func(rows *sql.Rows) error {
		var c Content
		err := rows.Scan(&c.Id, &c.Text, &c.Date)
		if err != nil {
			return err
		}
		ret = append(ret, c)
		return nil
	}

	if numLatest < 0 {
		numLatest = db.MaxVersions
	}

	err := db.query(query, row,
		sql.Named("path", path),
		sql.Named("limit", numLatest),
	)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (db *Db) Close() error {
	return db.db.Close()
}
