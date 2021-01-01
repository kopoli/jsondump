package main

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kopoli/appkit"
	"github.com/kopoli/jsondump/client"
	jsondump "github.com/kopoli/jsondump/server"
)

// type testOp interface {
// 	run(*Db) error
// }

// type testFunc func(*Db) error

// func (t testFunc) run(d *Db) error {
// 	return t(d)
// }

func TestOperations(t *testing.T) {

	dbfile := "test.sqlite3"
	opts := appkit.NewOptions()
	ctx := context.TODO()

	tests := []struct {
		name string
		wantErr bool
	}{
		// {"Improper parent directories", "nonexistent/file.db", true},
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

			cl, err := client.NewClient("")
			if err != nil {
				t.Errorf("Creating client failed with error = %v", err)
				return
			}

			cl.Http = srv.Client()

		})
	}

}
