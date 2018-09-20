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

func TestMetaImports(t *testing.T) {
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

	if url, err := MetaImports(context.Background(), hostname+"/foo/bar"); err != nil {
		t.Fatal(err)
	} else if url != "example.com/foo/bar" {
		t.Fatal(url)
	}
}

func TestMetaImportsExternal(t *testing.T) {
	if testing.Short() {
		t.Skip("testing with external VCS might be slow")
	}
	for _, test := range []struct {
		Pkg string
		URL string
	}{
		// Common VCS should be resolved immediately without any checks
		{Pkg: "github.com/user/repo", URL: "github.com/user/repo"},
		{Pkg: "bitbucket.org/user/repo", URL: "bitbucket.org/user/repo"},
		// Otherwise, HTML meta tag should be checked
		{Pkg: "golang.org/x/sys", URL: "go.googlesource.com/sys"},
		{Pkg: "gopkg.in/warnings.v0", URL: "gopkg.in/warnings.v0"},
		{Pkg: "gopkg.in/src-d/go-git.v4", URL: "gopkg.in/src-d/go-git.v4"},
		// On errors URL should be empty and error should be not nil
		{Pkg: "google.com/foo", URL: ""},
		{Pkg: "golang.org/x/sys/unix", URL: ""},
		{Pkg: "example.com/foo", URL: ""},
		{Pkg: "foo/bar", URL: ""},
	} {
		url, err := MetaImports(context.Background(), test.Pkg)
		if url != test.URL {
			t.Fatal(test, url, err)
		}
		if url == "" && err == nil {
			t.Fatal(test, "error should be set if module import can not be resolved")
		}
	}
}
