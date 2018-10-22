package vcs

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRepoRoot(t *testing.T) {
	var hostname string
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("go-get") != "1" {
			fmt.Fprint(w, `<!doctype html><html><body>Hello</body></html>`)
			return
		}
		fmt.Fprintf(w, `<!doctype html>
		<html>
		<head>
		<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
		<meta name="go-import" content="%s git https://example.com%s">
		</head>
		<body></body>
		</html>
		`, hostname+r.URL.Path, r.URL.Path)
	}))
	defer ts.Close()
	hostname = strings.TrimPrefix(ts.URL, "https://")

	if root, path, err := RepoRoot(context.Background(), hostname+"/foo/bar"); err != nil {
		t.Fatal(err)
	} else if root != "example.com/foo/bar" {
		t.Fatal(root)
	} else if path != "" {
		t.Fatal(path)
	}
}

func TestRepoRootExternal(t *testing.T) {
	if testing.Short() {
		t.Skip("testing with external VCS might be slow")
	}
	for _, test := range []struct {
		Pkg  string
		Root string
		Path string
	}{
		// Common VCS should be resolved immediately without any checks
		{Pkg: "github.com/user/repo", Root: "github.com/user/repo", Path: ""},
		{Pkg: "bitbucket.org/user/repo", Root: "bitbucket.org/user/repo", Path: ""},
		{Pkg: "github.com/user/repo/sub/dir", Root: "github.com/user/repo", Path: "sub/dir"},
		{Pkg: "bitbucket.org/user/repo/sub/dir", Root: "bitbucket.org/user/repo", Path: "sub/dir"},
		// Otherwise, HTML meta tag should be checked
		{Pkg: "golang.org/x/sys", Root: "go.googlesource.com/sys", Path: ""},
		{Pkg: "golang.org/x/sys/unix", Root: "go.googlesource.com/sys", Path: "unix"},
		{Pkg: "golang.org/x/net/websocket", Root: "go.googlesource.com/net", Path: "websocket"},
		{Pkg: "gopkg.in/warnings.v0", Root: "gopkg.in/warnings.v0", Path: ""},
		{Pkg: "gopkg.in/src-d/go-git.v4", Root: "gopkg.in/src-d/go-git.v4", Path: ""},
		// On errors URL should be empty and error should be not nil
		{Pkg: "google.com/foo", Root: "", Path: ""},
		{Pkg: "example.com/foo", Root: "", Path: ""},
		{Pkg: "foo/bar", Root: "", Path: ""},
	} {
		root, path, err := RepoRoot(context.Background(), test.Pkg)
		if root != test.Root {
			t.Fatal(test, root, err)
		} else if path != test.Path {
			t.Fatal(test, path, err)
		}
		if root == "" && path == "" && err == nil {
			t.Fatal(test, "error should be set if module import can not be resolved")
		}
	}
}
