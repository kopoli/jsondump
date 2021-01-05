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
	Op     int
}

type testOp interface {
	run(*state) error
}

type testFunc func(*state) error

func (t testFunc) run(s *state) error {
	return t(s)
}

func TestOperations(t *testing.T) {
	putraw := func(path string, content string) testFunc {
		return func(s *state) error {
			// i := json.RawMessage{}

			// err := json.Unmarshal([]byte(content), &i)
			// if err != nil {
			// 	return err
			// }

			return s.Client.PutRaw(path, []byte(content))
		}
	}

	del := func(path string) testFunc {
		return func(s *state) error {
			return s.Client.Delete(path)
		}
	}

	var failedOp int = -1

	expectFailure := func() testFunc {
		return func(s *state) error {
			if failedOp < 0 {
				e := fmt.Sprintf("Expected previous op (%d) to fail",
					s.Op-1)
				t.Errorf(e)
				return fmt.Errorf(e)
			}
			failedOp = -1
			return nil
		}
	}

	expectContent := func(path string, content ...string) testFunc {
		return func(s *state) error {
			d, err := s.Client.Get(path)
			if err != nil {
				return err
			}

			// content = clarify(content)

			// text := d.(string)
			return compare(t, "content not equal", content, d)
		}
	}

	dbfile := "integrate_test.sqlite3"
	opts := appkit.NewOptions()
	ctx := context.TODO()

	tests := []struct {
		name string
		ops  []testOp
	}{
		{"No test operations", []testOp{}},
		{"Nothing put, get empty", []testOp{
			expectContent("/abc", []string{}...),
		}},
		{"Simple put/get", []testOp{
			putraw("/abc", `"contenthere"`),
			expectContent("/abc", `"contenthere"`),
		}},
		{"Simple put/get 2", []testOp{
			putraw("/abc", `{"contenthere":   "first"   }`),
			expectContent("/abc", `{"contenthere":"first"}`),
		}},
		{"Put multiple", []testOp{
			putraw("/abc", `{"a":"b"   }`),
			putraw("/cde", `{"c":"d"   }`),
			expectContent("/abc", `{"a":"b"}`),
			expectContent("/cde", `{"c":"d"}`),
		}},
		{"Put hierarchy", []testOp{
			putraw("/abc/a", `{"a":"b"   }`),
			putraw("/abc/b", `{"c":"d"   }`),
			expectContent("/abc", `{"a":"b"}`, `{"c":"d"}`),
		}},
		{"Put overwrite", []testOp{
			putraw("/abc/a", `{"a":"b"   }`),
			putraw("/abc/a", `{"c":"d"   }`),
			expectContent("/abc", `{"c":"d"}`),
		}},
		{"Put invalid json", []testOp{
			putraw("/abc", `{"contenthere":"firs`),
			expectFailure(),
			expectContent("/abc", []string{}...),
		}},
		{"Delete empty", []testOp{
			del("/abc"),
			expectContent("/abc", []string{}...),
		}},
		{"Delete data", []testOp{
			putraw("/abc", `{"contenthere":   "first"   }`),
			expectContent("/abc", `{"contenthere":"first"}`),
			del("/abc"),
			expectContent("/abc", []string{}...),
		}},
		{"Delete hierarchy", []testOp{
			putraw("/abc/a", `{"a":"b"   }`),
			putraw("/abc/b", `{"c":"d"   }`),
			del("/abc"),
			expectContent("/abc", []string{}...),
		}},
		{"Delete hierarchy partly", []testOp{
			putraw("/abc/a", `{"a":"b"   }`),
			putraw("/abc/b", `{"c":"d"   }`),
			del("/abc/a"),
			expectContent("/abc", `{"c":"d"}`),
		}},
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
				st.Op = i
				err := op.run(st)
				failed = failed || (err != nil)
				if failed && !fail.failed {
					fail.failed = true
					fail.err = err
					fail.i = i
					failedOp = i
				}
			}
			if failed && failedOp >= 0 {
				t.Errorf("Unexpected error in op no.%d error = %v",
					fail.i, fail.err)
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
