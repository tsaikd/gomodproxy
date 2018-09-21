package vcs

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"testing"
)

func TestGit(t *testing.T) {
	if testing.Short() {
		t.Skip("testing with external VCS might be slow")
	}
	for _, test := range []struct {
		Module    string
		Tag       string
		Timestamp string
		Checksum  string
		Private   bool
	}{
		{
			// Repository with no tags and only a single master branch
			Module:    "bitbucket.org/gomodproxytest/head",
			Tag:       "v0.0.0-20180921102730-cbd3c6886e0f",
			Timestamp: "2018-09-21",
			Checksum:  "RpG4dddANe2fu9Ebi0nER6ZJTfZDGgKx+yKOabKdljA=",
		},
		{
			// Repository that uses lightweight tags
			Module:    "bitbucket.org/gomodproxytest/tags",
			Tag:       "v1.0.0",
			Timestamp: "2018-09-21",
			Checksum:  "YOW4xf9px088oQ+OP4xgRqjO8mk6eQ5vz7/kY+0Plqo=",
		},
		{
			// Repository that uses annotated tags
			Module:    "bitbucket.org/gomodproxytest/annotated",
			Tag:       "v1.0.0",
			Timestamp: "2018-09-21",
			Checksum:  "e7FcVp32XxvewwsiUuyU1Z06JjcEMnQvK5uBBw6kCR8=",
		},
		{
			// Repository that contains vendor directory
			Module:    "bitbucket.org/gomodproxytest/vendor",
			Tag:       "v1.0.0",
			Timestamp: "2018-09-21",
			Checksum:  "YIusKhLwlEKiuwowBFdEElpP0hIGDOCaSnQaMufLB00=",
		},
		{
			// Just a frequently used module from github
			Module:    "github.com/pkg/errors",
			Tag:       "v0.8.0",
			Timestamp: "2016-09-29",
			Checksum:  "WdK/asTD0HN+q6hsWO3/vpuAkAr+tw6aNJNDFFf0+qw=",
		},
	} {
		if test.Module == "" {
			continue
		}
		auth := NoAuth()
		if test.Tag != "" {
			t.Run(test.Module+"/List", func(t *testing.T) {
				git := NewGit(t.Log, test.Module, auth)
				list, err := git.List(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				for _, version := range list {
					if string(version) == test.Tag {
						return
					}
				}
				t.Fatal("tag not found")
			})
		}
		if test.Timestamp != "" {
			t.Run(test.Module+"/Timestamp", func(t *testing.T) {
				git := NewGit(t.Log, test.Module, auth)
				timestamp, err := git.Timestamp(context.Background(), Version(test.Tag))
				if err != nil {
					t.Fatal(err)
				}
				if timestamp.Format("2006-01-02") != test.Timestamp {
					t.Fatal(timestamp, test.Timestamp)
				}
			})
		}
		if test.Checksum != "" {
			t.Run(test.Module+"/ZIP", func(t *testing.T) {
				git := NewGit(t.Log, test.Module, auth)
				r, err := git.Zip(context.Background(), Version(test.Tag))
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				b, err := ioutil.ReadAll(r)
				if err != nil {
					t.Fatal(err)
				}
				h := sha256.New()
				zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
				if err != nil {
					t.Fatal(err)
				}
				fileSet := map[string]*zip.File{}
				fileList := []string{}
				for _, zf := range zr.File {
					fileSet[zf.Name] = zf
					fileList = append(fileList, zf.Name)
				}
				sort.Strings(fileList)
				for _, name := range fileList {
					f, err := fileSet[name].Open()
					if err != nil {
						t.Fatal(name, err)
					}
					defer f.Close()
					hf := sha256.New()
					io.Copy(hf, f)
					fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), name)
				}
				if cksum := base64.StdEncoding.EncodeToString(h.Sum(nil)); cksum != test.Checksum {
					t.Fatal(cksum, test.Checksum)
				}
			})
		}
	}
}
