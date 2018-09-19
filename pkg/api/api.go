package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/sixt/gomodproxy/pkg/store"
	"github.com/sixt/gomodproxy/pkg/vcs"
)

type logger = func(v ...interface{})

type api struct {
	log      logger
	vcsPaths []vcsPath
	stores   []store.Store
}

type vcsPath struct {
	prefix string
	vcs    func(module string) vcs.VCS
}

type Option func(*api)

var (
	apiList = regexp.MustCompile(`^/(?P<module>.*)/@v/list$`)
	apiInfo = regexp.MustCompile(`^/(?P<module>.*)/@v/(?P<version>.*).info$`)
	apiMod  = regexp.MustCompile(`^/(?P<module>.*)/@v/(?P<version>.*).mod$`)
	apiZip  = regexp.MustCompile(`^/(?P<module>.*)/@v/(?P<version>.*).zip$`)
)

func New(options ...Option) http.Handler {
	api := &api{log: func(...interface{}) {}}
	for _, opt := range options {
		opt(api)
	}
	return api
}

func Log(log logger) Option { return func(api *api) { api.log = log } }

func Git(prefix string, auth string) Option {
	a := vcs.Key(auth)
	if creds := strings.SplitN(auth, ":", 2); len(creds) == 2 {
		a = vcs.Password(creds[0], creds[1])
	}
	return func(api *api) {
		api.vcsPaths = append(api.vcsPaths, vcsPath{
			prefix: prefix,
			vcs: func(module string) vcs.VCS {
				return vcs.NewGit(api.log, module, a)
			},
		})
	}
}

func Memory() Option {
	return func(api *api) {
		api.stores = append(api.stores, store.Memory())
	}
}

func CacheDir(dir string) Option {
	return func(api *api) {
		api.stores = append(api.stores, store.Disk(dir))
	}
}

func decodeBangs(s string) string {
	buf := []rune{}
	bang := false
	for _, r := range []rune(s) {
		if bang {
			bang = false
			buf = append(buf, unicode.ToUpper(r))
			continue
		}
		if r == '!' {
			bang = true
			continue
		}
		buf = append(buf, r)
	}
	return string(buf)
}

func (api *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	defer func() { api.log("api.ServeHTTP", "method", r.Method, "url", r.URL, "time", time.Since(now)) }()

	for _, route := range []struct {
		regexp  *regexp.Regexp
		handler func(w http.ResponseWriter, r *http.Request, module, version string)
	}{
		{apiList, api.list},
		{apiInfo, api.info},
		{apiMod, api.mod},
		{apiZip, api.zip},
	} {
		if m := route.regexp.FindStringSubmatch(r.URL.Path); m != nil {
			module, version := m[1], ""
			if len(m) > 2 {
				version = m[2]
			}
			module = decodeBangs(module)
			route.handler(w, r, module, version)
			return
		}
	}

	http.NotFound(w, r)
}

func (api *api) vcs(ctx context.Context, module string) vcs.VCS {
	for _, path := range api.vcsPaths {
		if strings.HasPrefix(module, path.prefix) {
			return path.vcs(module)
		}
	}
	return vcs.NewGit(api.log, module, vcs.NoAuth())
}

func (api *api) module(ctx context.Context, module string, version vcs.Version) ([]byte, time.Time, error) {
	for _, store := range api.stores {
		if snapshot, err := store.Get(ctx, module, version); err == nil {
			return snapshot.Data, snapshot.Timestamp, nil
		}
	}

	timestamp, err := api.vcs(ctx, module).Timestamp(ctx, version)
	if err != nil {
		return nil, time.Time{}, err
	}

	b := &bytes.Buffer{}
	zr, err := api.vcs(ctx, module).Zip(ctx, version)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer zr.Close()

	if _, err := io.Copy(b, zr); err != nil {
		return nil, time.Time{}, err
	}

	for i := len(api.stores) - 1; i >= 0; i-- {
		err := api.stores[i].Put(ctx, store.Snapshot{
			Module:    module,
			Version:   version,
			Timestamp: timestamp,
			Data:      b.Bytes(),
		})
		if err != nil {
			api.log("api.module.Put", "module", module, "version", version, "error", err)
		}
	}

	return b.Bytes(), timestamp, nil
}

func (api *api) list(w http.ResponseWriter, r *http.Request, module, version string) {
	api.log("api.list", "module", module)
	list, err := api.vcs(r.Context(), module).List(r.Context())
	if err != nil {
		api.log("api.list", "module", module, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, v := range list {
		fmt.Fprintln(w, string(v))
	}
}

func (api *api) info(w http.ResponseWriter, r *http.Request, module, version string) {
	api.log("api.info", "module", module, "version", version)
	_, t, err := api.module(r.Context(), module, vcs.Version(version))

	if err != nil {
		api.log("api.info", "module", module, "version", version, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(struct {
		Version string
		Time    time.Time
	}{version, t})
}

func (api *api) mod(w http.ResponseWriter, r *http.Request, module, version string) {
	api.log("api.mod", "module", module, "version", version)
	b, _, err := api.module(r.Context(), module, vcs.Version(version))
	if err == nil {
		if zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b))); err == nil {
			for _, f := range zr.File {
				if f.Name == filepath.Join(module+"@"+string(version), "go.mod") {
					if r, err := f.Open(); err == nil {
						defer r.Close()
						io.Copy(w, r)
						return
					}
				}
			}
		}
	}
	w.Write([]byte(fmt.Sprintf("module %s\n", module)))
}

func (api *api) zip(w http.ResponseWriter, r *http.Request, module, version string) {
	api.log("api.zip", "module", module, "version", version)
	b, _, err := api.module(r.Context(), module, vcs.Version(version))
	if err != nil {
		api.log("api.zip", "module", module, "version", version, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(w, bytes.NewReader(b))
}
