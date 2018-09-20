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
					return "", errPrefixDoesNotMatch
				}
				url := f[2]
				if i := strings.Index(url, "://"); i >= 0 {
					url = url[i+3:]
				}
				return url, nil
			}
		}
	}
	return "", errMetaNotFound
}
