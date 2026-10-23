// Package testkit builds synthetic machines on disk for tests. It never
// touches the network; real data stays in .local-mirror.
package testkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/policy"
	"afvi/tools/core/internal/vfs"
)

// Env is an ops.Env over a map of nodes.
type Env map[policy.Ref]model.Store

func (e Env) Store(_ context.Context, r policy.Ref) (model.Store, error) {
	if s, ok := e[r]; ok {
		return s, nil
	}
	return model.Store{}, os.ErrNotExist
}

// NewStore creates an empty node (both shares) below dir.
func NewStore(t testing.TB, dir string) model.Store {
	t.Helper()
	for _, sh := range []string{"PxRepository", "PxInventory"} {
		if err := os.MkdirAll(filepath.Join(dir, sh), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return model.Store{
		Repo: vfs.NewOS(filepath.Join(dir, "PxRepository"), false),
		Inv:  vfs.NewOS(filepath.Join(dir, "PxInventory"), false),
	}
}

// Write creates a file with content, making parents.
func Write(t testing.TB, f vfs.FS, p, content string) {
	t.Helper()
	if err := f.MkdirAll(vfs.Join(p, "..")); err != nil {
		t.Fatal(err)
	}
	w, err := f.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// Read returns a file's content.
func Read(t testing.TB, f vfs.FS, p string) string {
	t.Helper()
	r, err := f.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b := make([]byte, 1<<20)
	n, _ := r.Read(b)
	return string(b[:n])
}

// AddModel creates a complete model: repository tree, .his, INSPECT_SPEC -00
// and LIGHT_SPEC. tag is written into the files so copies can be told apart.
func AddModel(t testing.TB, s model.Store, name, tag string) {
	t.Helper()
	Write(t, s.Repo, name+"/2.0/Line/Top/Top_handler.txt", "handler "+tag)
	Write(t, s.Repo, name+"/2.0/Line/Top/Layer/L01/1.tif", "tif1 "+tag)
	Write(t, s.Repo, name+"/2.0/Line/Bottom/Layer/L01/2.tif", "tif2 "+tag)
	Write(t, s.Repo, name+".his", name+",0,0,1,Admin,20260507114036\r\n"+name+",0,0,2,Admin,20260508114036\r\n")
	Write(t, s.Inv, "INSPECT_SPEC/"+name+"-00/TOP/LIGHT0/InspectionSpec.xml", "spec "+tag)
	Write(t, s.Inv, "LIGHT_SPEC/"+name+"/LightSpec.xml", "light "+tag)
}

// Exists reports whether p exists in f.
func Exists(f vfs.FS, p string) bool {
	_, err := f.Stat(p)
	return err == nil
}
