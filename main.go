package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kopoli/appkit"
	jsondump "github.com/kopoli/jsondump/server"
)

var (
	version     = "Undefined"
	timestamp   = "Undefined"
	buildGOOS   = "Undefined"
	buildGOARCH = "Undefined"
	progVersion = "" + version
)

func main() {
	checkErr := func(err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed with error %v\n", err)
			os.Exit(1)
		}
	}

	var err error

	dbpath := "testdb"
	opts := appkit.NewOptions()
	opts.Set("program-name", os.Args[0])
	opts.Set("program-version", progVersion)
	opts.Set("program-timestamp", timestamp)
	opts.Set("program-buildgoos", buildGOOS)
	opts.Set("program-buildgoarch", buildGOARCH)

	base := appkit.NewCommand(nil, "", "")
	base.Flags.Usage = func() {
		out := base.Flags.Output()
		fmt.Fprintf(out,
			"%s: Store json dumps\n\nCommands:\n",
			os.Args[0])
		base.CommandList(out)
		if appkit.HasFlags(base.Flags) {
			fmt.Fprintf(out, "\nOptions:\n")
			base.Flags.PrintDefaults()
		}
	}
	optVerbose := base.Flags.Bool("verbose", false, "Enable verbose output")
	optVersion := base.Flags.Bool("version", false, "Display version")
	optDbPath := base.Flags.String("db-path", dbpath, "Database path")

	web := appkit.NewCommand(base, "start-web web", "Start web server")
	optAddr := web.Flags.String("address", ":8042", "Listen address and port")
	optTimestampLog := web.Flags.Bool("log-timestamps", false, "Write timestamps to log")

	err = base.Parse(os.Args[1:], opts)
	if err == flag.ErrHelp {
		os.Exit(0)
	}
	checkErr(err)

	if *optVerbose {
		opts.Set("verbose", "t")
	}

	if *optVersion {
		fmt.Println(appkit.VersionString(opts))
		os.Exit(0)
	}

	dbpath = *optDbPath
	err = os.MkdirAll(dbpath, 0755)
	checkErr(err)

	cmd := opts.Get("cmdline-command", "")
	// args := SplitArguments(opts.Get("cmdline-args", ""))

	ctx := context.Background()

	dbpath = filepath.Join(dbpath, "jsondump.sqlite3")
	db, err := jsondump.CreateDb(dbpath, ctx)
	checkErr(err)
	defer db.Close()

	switch cmd {
	case "start-web":
		opts.Set("address", *optAddr)
		if *optTimestampLog {
			opts.Set("log-timestamps", "t")
		}
		err = jsondump.StartWeb(db, opts)
		checkErr(err)
		return
	}
}
