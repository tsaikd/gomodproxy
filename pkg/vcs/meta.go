package vcs

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"strings"
)

var (
	errPrefixDoesNotMatch = errors.New("prefix does not match the module")
	errMetaNotFound       = errors.New("go-import meta tag not found")
)

func RepoRoot(ctx context.Context, module string) (root string, path string, err error) {
	// For common VCS hosters we can figure out repo root by the URL
	if strings.HasPrefix(module, "github.com/") || strings.HasPrefix(module, "bitbucket.org/") {
		parts := strings.Split(module, "/")
		if len(parts) < 3 {
			return "", "", errors.New("bad module name")
		}
		return strings.Join(parts[0:3], "/"), strings.Join(parts[3:], "/"), nil
	}
	// Otherwise we shall make a `?go-get=1` HTTP request
	// TODO: use context
	res, err := http.Get("https://" + module + "?go-get=1")
	if err != nil {
		return "", "", err
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
		return "", "", err
	}
	for _, meta := range html.Head.Meta {
		if meta.Name == "go-import" {
			if f := strings.Fields(meta.Content); len(f) == 3 {
				url := f[2]
				if i := strings.Index(url, "://"); i >= 0 {
					url = url[i+3:]
				}
				path = strings.TrimPrefix(strings.TrimPrefix(module, f[0]), "/")
				return url, path, nil
			}
		}
	}
	return "", "", errMetaNotFound
}
