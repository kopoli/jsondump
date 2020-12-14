package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/pmezard/go-difflib/difflib"
)

// general testing functionality

func structEquals(a, b interface{}) bool {
	return spew.Sdump(a) == spew.Sdump(b)
}

func diffStr(a, b interface{}) (ret string) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(spew.Sdump(a)),
		B:        difflib.SplitLines(spew.Sdump(b)),
		FromFile: "Expected",
		ToFile:   "Received",
		Context:  3,
	}

	ret, _ = difflib.GetUnifiedDiffString(diff)
	return
}

func compare(t *testing.T, msg string, a, b interface{}) error {
	if !structEquals(a, b) {
		t.Error(msg, "\n", diffStr(a, b))
		return fmt.Errorf("Not equals")
	}
	return nil
}

var dbfile = "test.sqlite3"

func TestCreateDb(t *testing.T) {
	ctx := context.TODO()
	tests := []struct {
		name    string
		dbfile  string
		wantErr bool
	}{
		{"Improper parent directories", "nonexistent/file.db", true},
		{"Is a directory", ".", true},
		{"Proper filename", "test.sqlite3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet, err := CreateDb(tt.dbfile, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("openDbFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if gotRet == nil {
				t.Errorf("openDbFile() returns nil and no error")
				return
			}
			err = gotRet.Close()
			if err != nil {
				t.Errorf("db.Close() error = %v", err)
			}
			_, err = os.Stat(tt.dbfile)
			if err != nil {
				t.Errorf("Statting %s errors = %v", tt.dbfile, err)
			}
		})
	}

}

type testOp interface {
	run(*Db) error
}

type testFunc func(*Db) error

func (t testFunc) run(d *Db) error {
	return t(d)
}

func TestDb(t *testing.T) {
	add := func(path string, content ...string) testFunc {
		return func(d *Db) error {
			for _, c := range content {
				err := d.Add(path, c)
				if err != nil {
					return err
				}
			}
			return nil
		}
	}

	del := func(path string) testFunc {
		return func(d *Db) error {
			return d.Delete(path)
		}
	}

	expectLatestContent := func(path, content string) testFunc {
		return func(d *Db) error {
			c, err := d.GetContent(path, 1)
			if err != nil {
				return err
			}
			return compare(t, "content not equal", c[0].Text, content)
		}
	}

	setMaxVersions := func(vers int) testFunc {
		return func(d *Db) error {
			d.MaxVersions = vers
			return nil
		}
	}

	expectContentVersions := func(path string, count int) testFunc {
		return func(d *Db) error {
			c, err := d.GetContent(path, -1)
			if err != nil {
				return err
			}
			return compare(t, "Version count inequal", len(c), count)
		}
	}

	ctx := context.TODO()

	tests := []struct {
		name      string
		ops       []testOp
		wantErr   bool
		wantPaths []string
	}{
		{"Empty database", []testOp{}, false, []string{}},
		{"Single path", []testOp{
			add("/abc", "content"),
		}, false, []string{"/abc"}},
		{"Two paths", []testOp{
			add("/abc", "content"),
			add("/second", "other"),
		}, false, []string{"/abc", "/second"}},
		{"Data", []testOp{
			add("/a", "content"),
			expectLatestContent("/a", "content"),
			add("/a", "updated"),
			expectLatestContent("/a", "updated"),
			add("/a", "Third time"),
			expectLatestContent("/a", "Third time"),
		}, false, []string{"/a"}},
		{"Versions under limit", []testOp{
			setMaxVersions(5),
			add("/a", "1", "2", "3", "4"),
			expectLatestContent("/a", "4"),
			expectContentVersions("/a", 4),
		}, false, []string{"/a"}},
		{"Versions over limit", []testOp{
			setMaxVersions(5),
			add("/a", "1", "2", "3", "4", "5", "6", "7"),
			expectLatestContent("/a", "7"),
			expectContentVersions("/a", 5),
		}, false, []string{"/a"}},
		{"Default versions over limit", []testOp{
			add("/a", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"),
			expectLatestContent("/a", "11"),
			expectContentVersions("/a", 10),
		}, false, []string{"/a"}},
		{"Deleting one path", []testOp{
			add("/abc", "content"),
			del("/abc"),
		}, false, []string{}},
		{"Deleting one path 2", []testOp{
			add("/abc", "content"),
			add("/second", "other"),
			add("/third", "val"),
			del("/second"),
		}, false, []string{"/abc", "/third"}},
	}
	for _, tt := range tests {
		// Remove the dbfile before testing
		_ = os.Remove(dbfile)

		db, err := CreateDb(dbfile, ctx)
		if err != nil {
			t.Errorf("Setting up db failed with error = %v", err)
			return
		}

		t.Run(tt.name, func(t *testing.T) {
			var failed bool = false
			fail := struct {
				failed bool
				err    error
				i      int
			}{}
			for i, op := range tt.ops {
				err := op.run(db)
				failed = failed || (err != nil)
				if failed && !fail.failed {
					fail.failed = true
					fail.err = err
					fail.i = i
				}
			}
			if failed != tt.wantErr {
				t.Errorf("op no.%d error = %v, wantErr %v", fail.i, fail.err, tt.wantErr)
				return
			}
			paths, err := db.GetPaths()
			if err != nil {
				t.Errorf("Getting paths failed with error = %v",
					err)
				return
			}
			_ = compare(t, "db.GetPaths not expected", paths, tt.wantPaths)
		})
		err = db.Close()
		if err != nil {
			t.Errorf("Closing db failed with error = %v", err)
			return
		}

	}
}
