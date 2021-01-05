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

type testData struct {
	A int
	B string
}

func TestOperations(t *testing.T) {
	putRaw := func(path string, content string) testFunc {
		return func(s *state) error {
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

	expectRawContent := func(path string, content ...string) testFunc {
		return func(s *state) error {
			d, err := s.Client.GetRaw(path)
			if err != nil {
				return err
			}

			// content = clarify(content)

			// text := d.(string)
			return compare(t, "content not equal", content, d)
		}
	}

	put := func(path string, content interface{}) testFunc {
		return func(s *state) error {
			return s.Client.Put(path, content)
		}
	}

	expectContent := func(path string, content ...testData) testFunc {
		return func(s *state) error {
			var v []testData
			err := s.Client.Get(path, &v)
			if err != nil {
				return err
			}

			// Clear extra slice capacity
			v2 := make([]testData,len(v))
			copy(v2, v)

			return compare(t, "content not equal", content, v2)
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
			expectRawContent("/abc", []string{}...),
		}},
		{"Simple put/get", []testOp{
			putRaw("/abc", `"contenthere"`),
			expectRawContent("/abc", `"contenthere"`),
		}},
		{"Simple put/get 2", []testOp{
			putRaw("/abc", `{"contenthere":   "first"   }`),
			expectRawContent("/abc", `{"contenthere":"first"}`),
		}},
		{"Put multiple", []testOp{
			putRaw("/abc", `{"a":"b"   }`),
			putRaw("/cde", `{"c":"d"   }`),
			expectRawContent("/abc", `{"a":"b"}`),
			expectRawContent("/cde", `{"c":"d"}`),
		}},
		{"Put hierarchy", []testOp{
			putRaw("/abc/a", `{"a":"b"   }`),
			putRaw("/abc/b", `{"c":"d"   }`),
			expectRawContent("/abc", `{"a":"b"}`, `{"c":"d"}`),
		}},
		{"Put overwrite", []testOp{
			putRaw("/abc/a", `{"a":"b"   }`),
			putRaw("/abc/a", `{"c":"d"   }`),
			expectRawContent("/abc", `{"c":"d"}`),
		}},
		{"Put invalid json", []testOp{
			putRaw("/abc", `{"contenthere":"firs`),
			expectFailure(),
			expectRawContent("/abc", []string{}...),
		}},
		{"Delete empty", []testOp{
			del("/abc"),
			expectRawContent("/abc", []string{}...),
		}},
		{"Delete data", []testOp{
			putRaw("/abc", `{"contenthere":   "first"   }`),
			expectRawContent("/abc", `{"contenthere":"first"}`),
			del("/abc"),
			expectRawContent("/abc", []string{}...),
		}},
		{"Delete hierarchy", []testOp{
			putRaw("/abc/a", `{"a":"b"   }`),
			putRaw("/abc/b", `{"c":"d"   }`),
			del("/abc"),
			expectRawContent("/abc", []string{}...),
		}},
		{"Delete hierarchy partly", []testOp{
			putRaw("/abc/a", `{"a":"b"   }`),
			putRaw("/abc/b", `{"c":"d"   }`),
			del("/abc/a"),
			expectRawContent("/abc", `{"c":"d"}`),
		}},
		{"Put with marshalling", []testOp{
			put("/abc", testData{A: 10, B: "smth"}),
			expectContent("/abc", testData{A: 10, B: "smth"}),
		}},
		{"Put with marshalling hierarchy", []testOp{
			put("/abc/a", testData{A: 10, B: "smth"}),
			put("/abc/b", testData{A: -1, B: "val"}),
			expectContent("/abc", testData{A: 10, B: "smth"},
				testData{A: -1, B: "val"}),
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
