package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/kopoli/appkit"
	"github.com/kopoli/jsondump/client"
	jsondump "github.com/kopoli/jsondump/server"
	"github.com/pmezard/go-difflib/difflib"
)

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

type state struct {
	Db     *jsondump.Db
	Client *client.Client
}

type testOp interface {
	run(*state) error
}

type testFunc func(*state) error

func (t testFunc) run(s *state) error {
	return t(s)
}

func TestOperations(t *testing.T) {
	put := func(path string, content string) testFunc {
		return func(s *state) error {
			return s.Client.Put(path, content)
		}
	}

	expectContent := func(path string, content ...string) testFunc {
		return func(s *state) error {
			d, err := s.Client.Get(path)
			if err != nil {
				return err
			}

			return compare(t, "content not equal", d, content)
		}
	}

	dbfile := "integrate_test.sqlite3"
	opts := appkit.NewOptions()
	ctx := context.TODO()

	tests := []struct {
		name    string
		ops     []testOp
		wantErr bool
	}{
		{"No test operations", []testOp{}, false},
		{"Nothing put, get empty", []testOp{
			expectContent("/abc", []string{}...),
		}, false},
		{"Simple put/get", []testOp{
			put("/abc", `"contenthere"`),
			expectContent("/abc", `"contenthere"`),
		}, false},
		{"Simple put/get 2", []testOp{
			put("/abc", `{"contenthere": "first"}`),
			expectContent("/abc", `{"contenthere": "first"}`),
		}, false},
	}
	for _, tt := range tests {
		_ = os.Remove(dbfile)
		db, err := jsondump.CreateDb(dbfile, ctx)
		if err != nil {
			t.Errorf("Setting up db failed with error = %v", err)
			return
		}

		t.Run(tt.name, func(t *testing.T) {
			mux := jsondump.CreateHandler(db, opts)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			cl, err := client.NewClient(srv.URL, opts)
			if err != nil {
				t.Errorf("Creating client failed with error = %v", err)
				return
			}

			cl.Http = srv.Client()

			st := &state{
				Db:     db,
				Client: cl,
			}

			var failed bool = false
			fail := struct {
				failed bool
				err    error
				i      int
			}{}
			for i, op := range tt.ops {
				err := op.run(st)
				failed = failed || (err != nil)
				if failed && !fail.failed {
					fail.failed = true
					fail.err = err
					fail.i = i
				}
			}
			if failed != tt.wantErr {
				t.Errorf("op no.%d error = %v, wantErr %v",
					fail.i, fail.err, tt.wantErr)
				return
			}
		})
		err = db.Close()
		if err != nil {
			t.Errorf("Closing db failed with error = %v", err)
			return
		}
	}

}
