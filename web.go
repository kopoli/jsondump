package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"strings"
	"sync"
	"time"

	"github.com/kopoli/appkit"
)

type middleware func(http.Handler) http.Handler

func chain(h http.Handler, m ...middleware) http.Handler {
	for i := range m {
		h = m[len(m)-1-i](h)
	}

	return h
}

type CodeResponseWriter struct {
	http.ResponseWriter
	Code int
	Len  int
}

func (w *CodeResponseWriter) WriteHeader(code int) {
	w.Code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *CodeResponseWriter) Write(b []byte) (int, error) {
	l, err := w.ResponseWriter.Write(b)
	w.Len += l
	return l, err
}

func logHandler() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			cw := &CodeResponseWriter{w, 0, 0}
			next.ServeHTTP(cw, r)
			dur := time.Since(start)
			log.Printf("%s %d %s %d %s %s", r.RemoteAddr, cw.Code, dur.String(), cw.Len, r.Method, r.URL.String())
		})
	}
}

type RestApi struct {
	prefix  string
	db      *Db
	dbMutex sync.RWMutex
	version string
}

func wrapJson(data string, err error) []byte {
	status := "success"
	if err != nil {
		errstr := strings.Replace(err.Error(), `"`, `\"`, -1)
		data = fmt.Sprintf(`"%v"`, errstr)
		status = "fail"
	} else if data == "" {
		data = `""`
	}

	ret := fmt.Sprintf(`{"status": "%s", "data": %s}`, status, data)

	return []byte(ret)
}

func respond(w http.ResponseWriter, data string, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	msg := wrapJson(data, err)
	w.WriteHeader(code)
	_, err = w.Write(msg)
	if err != nil {
		log.Printf("Write failed with %v", err)
	}
}

func (ra *RestApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// path := strings.TrimPrefix(r.URL.EscapedPath(), ra.prefix)

	// codeFromError := func(err error) int {
	// 	if err != nil {
	// 		return http.StatusBadRequest
	// 	} else {
	// 		return http.StatusOK
	// 	}
	// }

	switch r.Method {
	case "GET":
		// out, err := ra.db.Get(dbCmd, args...)
		ra.dbMutex.RLock()
		// respond(w, out, err, codeFromError(err))
		ra.dbMutex.RUnlock()
		// return
	case "PUT":
		// ra.dbMutex.Lock()
		// err := ra.db.Set(dbCmd, args...)
		// if err == nil {
		// 	err = ra.db.Export()
		// }
		// ra.dbMutex.Unlock()
		// respond(w, "", err, codeFromError(err))
		// return
	case "DELETE":
		// ra.dbMutex.Lock()
		// err := ra.db.Delete(dbCmd, args...)
		// if err == nil {
		// 	err = ra.db.Export()
		// }
		// ra.dbMutex.Unlock()
		// respond(w, "", err, codeFromError(err))
		// return
	default:
		respond(w, "", fmt.Errorf("Unknown method"), http.StatusBadRequest)
		return
	}
}

func StartWeb(db *Db, opts appkit.Options) error {
	var logflags int = 0

	if opts.IsSet("log-timestamps") {
		logflags = log.LstdFlags
	}

	log.SetFlags(logflags)

	r := &RestApi{
		prefix:  "/api/",
		db:      db,
		version: opts.Get("program-version", "undefined"),
	}
	mux := http.NewServeMux()

	stack := func(h http.Handler) http.Handler {
		return chain(h,
			logHandler(),
			// corsHandler(),
		)
	}

	mux.Handle(r.prefix, stack(r))

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// mux.Handle("/", stack(http.FileServer(_escDir(true, "/"))))
	// mux.Handle("/", stack(http.FileServer(http.Dir("static"))))

	addr := opts.Get("address", ":8042")

	log.Println("Starting server at", addr)

	srv := &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: 20 * time.Second,
		// WriteTimeout: 20 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	return srv.ListenAndServe()
}
