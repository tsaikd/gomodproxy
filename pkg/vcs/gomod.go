package vcs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type goVCS struct {
	dir    string
	log    logger
	module string
}

func NewGoMod(l logger, module string) VCS {
	return &goVCS{log: l, module: module, dir: "/tmp/_go"}
}

func (g *goVCS) List(ctx context.Context) ([]Version, error) {
	if err := g.download(ctx, "latest"); err != nil {
		return nil, err
	}
	b, err := g.file("list")
	if err != nil {
		return nil, err
	}
	versions := []Version{}
	for _, line := range strings.Split(string(b), "\n") {
		versions = append(versions, Version(line))
	}
	return versions, nil
}

func (g *goVCS) Timestamp(ctx context.Context, version Version) (time.Time, error) {
	if err := g.download(ctx, version.String()); err != nil {
		return time.Time{}, err
	}
	b, err := g.file(version.String() + ".info")
	if err != nil {
		return time.Time{}, err
	}
	info := struct {
		Version string
		Time    time.Time
	}{}
	if json.Unmarshal(b, &info) == nil {
		return info.Time, nil
	}
	if t, err := time.Parse(time.RFC3339, string(b)); err == nil {
		return t, nil
	}
	if sec, err := strconv.ParseInt(string(b), 10, 64); err == nil {
		return time.Unix(sec, 0), nil
	}
	return time.Time{}, nil
}

func (g *goVCS) Zip(ctx context.Context, version Version) (io.ReadCloser, error) {
	if err := g.download(ctx, version.String()); err != nil {
		return nil, err
	}
	b, err := g.file(version.String() + ".zip")
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(b)), nil
}

func (g *goVCS) download(ctx context.Context, version string) error {
	cmd := exec.Command("go", "mod", "download", g.module+"@"+version)
	cmd.Env = append(os.Environ(), "GOPATH="+g.dir)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (g *goVCS) file(name string) ([]byte, error) {
	path := filepath.Join(g.dir, "pkg", "mod", "cache", "download", g.module, "@v", name)
	data, err := ioutil.ReadFile(path)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if module, errMod := encodeString(g.module); errMod == nil {
			path = filepath.Join(g.dir, "pkg", "mod", "cache", "download", module, "@v", name)
			return ioutil.ReadFile(path)
		}
	}
	return data, err
}

// Ripped from cmd/go
func encodeString(s string) (encoding string, err error) {
	haveUpper := false
	for _, r := range s {
		if r == '!' || r >= utf8.RuneSelf {
			// This should be disallowed by CheckPath, but diagnose anyway.
			// The correctness of the encoding loop below depends on it.
			return "", fmt.Errorf("internal error: inconsistency in EncodePath")
		}
		if 'A' <= r && r <= 'Z' {
			haveUpper = true
		}
	}

	if !haveUpper {
		return s, nil
	}

	var buf []byte
	for _, r := range s {
		if 'A' <= r && r <= 'Z' {
			buf = append(buf, '!', byte(r+'a'-'A'))
		} else {
			buf = append(buf, byte(r))
		}
	}
	return string(buf), nil
}
