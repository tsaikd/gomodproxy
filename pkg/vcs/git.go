package vcs

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

const remoteName = "origin"

type gitVCS struct {
	log    logger
	module string
	auth   Auth
}

// NewGit return a go-git VCS client implementation that provides information
// about the specific module using the pgiven authentication mechanism.
func NewGit(l logger, module string, auth Auth) VCS {
	return &gitVCS{log: l, module: module, auth: auth}
}

func (g *gitVCS) List(ctx context.Context) ([]Version, error) {
	g.log("gitVCS.List", "module", g.module)
	repo, err := g.repo(ctx)
	if err != nil {
		return nil, err
	}

	remote, err := repo.Remote(remoteName)
	if err != nil {
		return nil, err
	}

	auth, err := g.authMethod()
	if err != nil {
		return nil, err
	}

	refs, err := remote.List(&git.ListOptions{Auth: auth})
	if err != nil {
		return nil, err
	}

	list := []Version{}
	masterHash := ""
	for _, ref := range refs {
		name := ref.Name()
		if name == plumbing.Master {
			masterHash = ref.Hash().String()
		} else if name.IsTag() && strings.HasPrefix(name.String(), "refs/tags/v") {
			list = append(list, Version(strings.TrimPrefix(name.String(), "refs/tags/")))
		}
	}

	if len(list) == 0 {
		if masterHash == "" {
			return nil, errors.New("no tags and no master branch found")
		}
		short := masterHash[:12]
		t, err := g.Timestamp(ctx, Version("v0.0.0-20060102150405-"+short))
		if err != nil {
			return nil, err
		}
		list = []Version{Version(fmt.Sprintf("v0.0.0-%s-%s", t.Format("20060102150405"), short))}
	}

	g.log("gitVCS.List", "module", g.module, "list", list)
	return list, nil
}

func (g *gitVCS) Timestamp(ctx context.Context, version Version) (time.Time, error) {
	g.log("gitVCS.Timestamp", "module", g.module, "version", version)
	ci, err := g.commit(ctx, version)
	if err != nil {
		return time.Time{}, err
	}
	g.log("gitVCS.Timestamp", "module", g.module, "version", version, "timestamp", ci.Committer.When)
	return ci.Committer.When, nil
}

func isVendoredPackage(name string) bool {
	var i int
	if strings.HasPrefix(name, "vendor/") {
		i += len("vendor/")
	} else if j := strings.Index(name, "/vendor/"); j >= 0 {
		i += len("/vendor/")
	} else {
		return false
	}
	return strings.Contains(name[i:], "/")
}

func (g *gitVCS) Zip(ctx context.Context, version Version) (io.ReadCloser, error) {
	g.log("gitVCS.Zip", "module", g.module, "version", version)
	ci, err := g.commit(ctx, version)
	if err != nil {
		return nil, err
	}
	tree, err := ci.Tree()
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	zw := zip.NewWriter(b)
	tree.Files().ForEach(func(f *object.File) error {
		// go mod strips vendored directories from the zip, and we do the same
		// to match the checksums in the go.sum
		if isVendoredPackage(f.Name) {
			return nil
		}
		w, err := zw.Create(filepath.Join(g.module+"@"+string(version), f.Name))
		if err != nil {
			return err
		}
		r, err := f.Reader()
		if err != nil {
			return err
		}
		defer r.Close()
		io.Copy(w, r)
		return nil
	})
	zw.Close()
	return ioutil.NopCloser(bytes.NewBuffer(b.Bytes())), nil
}

func (g *gitVCS) repo(ctx context.Context) (*git.Repository, error) {
	repo, err := git.Init(memory.NewStorage(), nil)
	if err != nil {
		return nil, err
	}
	schema := "https://"
	if g.auth.Key != "" {
		schema = "ssh://"
	}
	repoRoot := g.module
	if meta, err := MetaImports(ctx, g.module); err == nil {
		repoRoot = meta
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{schema + repoRoot + ".git"},
	})
	return repo, err
}

func (g *gitVCS) commit(ctx context.Context, version Version) (*object.Commit, error) {
	repo, err := g.repo(ctx)
	if err != nil {
		return nil, err
	}
	auth, err := g.authMethod()
	if err != nil {
		return nil, err
	}
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: remoteName,
		Auth:       auth,
	})
	if err != nil {
		return nil, err
	}

	version = Version(strings.TrimSuffix(string(version), "+incompatible"))
	hash := version.Hash()
	if version.IsSemVer() {
		tags, err := repo.Tags()
		if err != nil {
			return nil, err
		}
		tags.ForEach(func(t *plumbing.Reference) error {
			if t.Name().String() == "refs/tags/"+string(version) {
				hash = t.Hash().String()
				annotated, err := repo.TagObject(t.Hash())
				if err == nil {
					hash = annotated.Target.String()
				}
			}
			return nil
		})
	} else {
		commits, err := repo.CommitObjects()
		if err != nil {
			return nil, err
		}
		commits.ForEach(func(ci *object.Commit) error {
			if strings.HasPrefix(ci.Hash.String(), version.Hash()) {
				hash = ci.Hash.String()
			}
			return nil
		})
	}

	g.log("gitVCS.commit", "module", g.module, "version", version, "hash", hash)
	return repo.CommitObject(plumbing.NewHash(hash))
}

func (g *gitVCS) authMethod() (transport.AuthMethod, error) {
	if g.auth.Key != "" {
		return ssh.NewPublicKeysFromFile("git", g.auth.Key, "")
	} else if g.auth.Username != "" {
		return &http.BasicAuth{Username: g.auth.Username, Password: g.auth.Password}, nil
	}
	return nil, nil
}
