package filetime

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	testAccess = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	testModify = time.Date(2011, 12, 13, 14, 15, 16, 0, time.UTC)
)

func setTimes(t *testing.T, path string) {
	t.Helper()
	if err := os.Chtimes(path, testAccess, testModify); err != nil {
		t.Fatal(err)
	}
}

func assertTimes(t *testing.T, got Times) {
	t.Helper()
	if !got.Access.Equal(testAccess) || !got.Modify.Equal(testModify) {
		t.Fatalf("got %+v, want access=%s modify=%s", got, testAccess, testModify)
	}
}

func TestReadTimes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	setTimes(t, path)

	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	assertTimes(t, got)
}

func TestReadDirectoryAndLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	setTimes(t, target)

	t.Run("directory", func(t *testing.T) {
		got, err := Read(target)
		if err != nil {
			t.Fatal(err)
		}
		assertTimes(t, got)
	})

	t.Run("link", func(t *testing.T) {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("cannot create symbolic link: %v", err)
		}
		got, err := Read(link)
		if err != nil {
			t.Fatal(err)
		}
		assertTimes(t, got)
	})
}

func TestReadMissing(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("got error %v, want fs.ErrNotExist", err)
	}
}
