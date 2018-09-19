package vcs

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type logger = func(v ...interface{})

// Version represents a semantic version of a module.
type Version string

var reSemVer = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// IsSemVer returns true if a version string is a semantic version e.g. vX.Y.Z.
func (v Version) IsSemVer() bool { return reSemVer.MatchString(string(v)) }

// Hash returns a commit hash if a version is of a form v0.0.0-timestamp-hash.
func (v Version) Hash() string {
	fields := strings.Split(string(v), "-")
	if len(fields) != 3 {
		return ""
	}
	return fields[2]
}

// Module is a source code snapshot for which one can get the commit timestamp
// or the actual ZIP with the source code in it.
type Module interface {
	Timestamp(ctx context.Context, version Version) (time.Time, error)
	Zip(ctx context.Context, version Version) (io.ReadCloser, error)
}

// VCS is a version control system client. It can list available versions from
// the remote, as well as fetch module data such as timestamp or zip snapshot.
type VCS interface {
	List(ctx context.Context) ([]Version, error)
	Module
}

// Auth defines a typical VCS authentication mechanism, such as SSH key or
// username/password.
type Auth struct {
	Username string
	Password string
	Key      string
}

// NoAuth returns an Auth implementation that uses no authentication at all.
func NoAuth() Auth { return Auth{} }

// Password returns an Auth implementation that authenticate via username and password.
func Password(username, password string) Auth { return Auth{Username: username, Password: password} }

// Key returns an Auth implementation that uses key file authentication mechanism.
func Key(key string) Auth { return Auth{Key: key} }

// MetaImports resolved module import path for certain hosts using the special <meta> tag.
func MetaImports(ctx context.Context, module string) (string, error) {
	if strings.HasPrefix(module, "github.com/") || strings.HasPrefix(module, "bitbucket.org/") {
		return module, nil
	}
	// TODO: use context
	res, err := http.Get("https://" + module + "?go-get=1")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	html := struct {
		HTML string `xml:"html"`
		Head struct {
			Meta []struct {
				Name    string `xml:"name,attr"`
				Content string `xml:"content,attr"`
			} `xml:"meta"`
		} `xml:"head"`
	}{}
	dec := xml.NewDecoder(res.Body)
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	if err := dec.Decode(&html); err != nil {
		return "", err
	}
	for _, meta := range html.Head.Meta {
		if meta.Name == "go-import" {
			if f := strings.Fields(meta.Content); len(f) == 3 {
				if f[0] != module {
					return "", errors.New("prefix does not match the module")
				}
				url := f[2]
				if i := strings.Index(url, "://"); i >= 0 {
					url = url[i+3:]
				}
				return url, nil
			}
		}
	}
	return "", errors.New("go-import meta tag not found")
}
