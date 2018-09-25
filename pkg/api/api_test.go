package api

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const testGoSource = `
package main

import (
	_ "github.com/pkg/errors"
	_ "golang.org/x/net/websocket"
)

func main() {}
`

func TestBuildWithProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("testing with external VCS might be slow")
		return
	}

	// Start a proxy
	api := New(Log(t.Log), Memory(t.Log, 128*1024*1024))
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go (&http.Server{Handler: api}).Serve(ln)

	// Create temporary directory for a minimal test Go project
	tmpDir, err := ioutil.TempDir(os.TempDir(), "gomodproxy_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Cleanup requires changing the permissions, because some files generated
		// by "go build" are read-only.
		filepath.Walk(tmpDir, func(f string, fi os.FileInfo, err error) error {
			return os.Chmod(f, 0777)
		})
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Log("RemoveAll", tmpDir, err)
		}
	}()

	// Generate test main.go and go.mod
	err = ioutil.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(testGoSource), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module gomodproxy_test"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Perform Go build
	cmd := exec.Command("go", "build")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GOPATH="+filepath.Join(tmpDir, "_gopath"),
		"GOCACHE="+filepath.Join(tmpDir, "_gocache"),
		"GOPROXY=http://"+ln.Addr().String(),
		"GO111MODULE=on")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
	}
}
