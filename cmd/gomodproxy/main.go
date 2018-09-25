package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/sixt/gomodproxy/pkg/api"

	_ "expvar"
	_ "net/http/pprof"
)

func prettyLog(v ...interface{}) {
	s := ""
	msg := ""
	if len(v)%2 != 0 {
		msg = fmt.Sprintf("%s", v[0])
		v = v[1:]
	}
	s = fmt.Sprintf("%20s ", msg)
	for i := 0; i < len(v); i = i + 2 {
		s = s + fmt.Sprintf("%v=%v ", v[i], v[i+1])
	}
	log.Println(s)
}

func jsonLog(v ...interface{}) {
	entry := map[string]interface{}{}
	if len(v)%2 != 0 {
		entry["msg"] = v[0]
		v = v[1:]
	}
	for i := 0; i < len(v); i = i + 2 {
		entry[fmt.Sprintf("%v", v[i])] = v[i+1]
	}
	json.NewEncoder(os.Stdout).Encode(entry)
}

type listFlag []string

func (f *listFlag) String() string     { return strings.Join(*f, " ") }
func (f *listFlag) Set(s string) error { *f = append(*f, s); return nil }

func main() {
	gitPaths := listFlag{}

	addr := flag.String("addr", ":0", "http server address")
	verbose := flag.Bool("v", false, "verbose logging")
	debug := flag.Bool("debug", false, "enable debug HTTP API (pprof/expvar)")
	json := flag.Bool("json", false, "json structured logging")
	dir := flag.String("dir", filepath.Join(os.Getenv("HOME"), ".gomodproxy"), "cache directory")
	memLimit := flag.Int64("mem", 256, "in-memory cache size in MB")
	flag.Var(&gitPaths, "git", "list of git settings")

	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("net.Listen:", err)
	}
	defer ln.Close()

	fmt.Println("Listening on", ln.Addr())

	options := []api.Option{}
	logger := func(...interface{}) {}
	if *verbose || *json {
		if *json {
			logger = jsonLog
		} else {
			logger = prettyLog
		}
	}
	options = append(options, api.Log(logger))

	for _, path := range gitPaths {
		kv := strings.SplitN(path, "=", 2)
		if len(kv) != 2 {
			log.Fatal("bad git path:", path)
		}
		options = append(options, api.Git(kv[0], kv[1]))
	}

	options = append(options,
		api.Memory(logger, *memLimit*1024*1024),
		api.CacheDir(*dir),
	)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	mux := http.NewServeMux()
	mux.Handle("/", api.New(options...))
	if *debug {
		mux.Handle("/debug/vars", http.DefaultServeMux)
		mux.Handle("/debug/pprof/heap", http.DefaultServeMux)
		mux.Handle("/debug/pprof/profile", http.DefaultServeMux)
		mux.Handle("/debug/pprof/block", http.DefaultServeMux)
		mux.Handle("/debug/pprof/trace", http.DefaultServeMux)
	}

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil {
			log.Fatal(err)
		}
	}()

	<-sigc
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
