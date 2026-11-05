package touch

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

var (
	seedTimes = filetime.Times{
		Access: time.Date(2001, 1, 1, 1, 1, 1, 0, time.UTC),
		Modify: time.Date(2002, 2, 2, 2, 2, 2, 0, time.UTC),
	}
	requestTimes = filetime.Times{
		Access: time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC),
		Modify: time.Date(2022, 8, 9, 10, 11, 12, 0, time.UTC),
	}
)

func writeSeeded(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	seed(t, path)
}

func seed(t *testing.T, path string) {
	t.Helper()
	if err := os.Chtimes(path, seedTimes.Access, seedTimes.Modify); err != nil {
		t.Fatal(err)
	}
}

func readTimes(t *testing.T, path string) filetime.Times {
	t.Helper()
	times, err := filetime.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return times
}

func sameTimes(a, b filetime.Times) bool {
	return a.Access.Equal(b.Access) && a.Modify.Equal(b.Modify)
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	_, err := os.Lstat(path)
	if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("%s: got %v, want it to be absent", path, err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}
}

func assertBetween(t *testing.T, name string, got, start, end time.Time) {
	t.Helper()
	if got.Before(start.Truncate(time.Second)) || got.After(end.Add(time.Second)) {
		t.Errorf("%s: got %s, want between %s and %s", name, got, start, end)
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		wantAccess time.Time
		wantModify time.Time
	}{
		{"both", Options{}, requestTimes.Access, requestTimes.Modify},
		{"access and modify", Options{Access: true, Modify: true}, requestTimes.Access, requestTimes.Modify},
		{"access", Options{Access: true}, requestTimes.Access, seedTimes.Modify},
		{"modify", Options{Modify: true}, seedTimes.Access, requestTimes.Modify},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			writeSeeded(t, path, "content")

			if err := Update(path, requestTimes, tt.opts); err != nil {
				t.Fatal(err)
			}

			got := readTimes(t, path)
			if !got.Access.Equal(tt.wantAccess) || !got.Modify.Equal(tt.wantModify) {
				t.Errorf("got %+v, want access=%s modify=%s", got, tt.wantAccess, tt.wantModify)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "content" {
				t.Errorf("got content %q, want %q", content, "content")
			}
		})
	}
}

func TestUpdateDirectory(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	writeSeeded(t, child, "")
	seed(t, dir)

	if err := Update(dir, requestTimes, Options{}); err != nil {
		t.Fatal(err)
	}

	if got := readTimes(t, dir); !sameTimes(got, requestTimes) {
		t.Errorf("directory: got %+v, want %+v", got, requestTimes)
	}
	if got := readTimes(t, child); !got.Access.Equal(seedTimes.Access) || !got.Modify.Equal(seedTimes.Modify) {
		t.Errorf("child: got %+v, want unchanged %+v", got, seedTimes)
	}
}

func TestUpdateCreates(t *testing.T) {
	tests := []struct {
		name         string
		opts         Options
		wantAccess   bool
		wantModify   bool
		wantCreation string
	}{
		{"both", Options{}, true, true, ""},
		{"access", Options{Access: true}, true, false, "modify"},
		{"modify", Options{Modify: true}, false, true, "access"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "new")

			start := time.Now()
			if err := Update(path, requestTimes, tt.opts); err != nil {
				t.Fatal(err)
			}
			end := time.Now()

			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			got := readTimes(t, path)
			if !info.Mode().IsRegular() || info.Size() != 0 {
				t.Errorf("got mode %s size %d, want an empty regular file", info.Mode(), info.Size())
			}
			if tt.wantAccess && !got.Access.Equal(requestTimes.Access) {
				t.Errorf("access: got %s, want %s", got.Access, requestTimes.Access)
			}
			if tt.wantModify && !got.Modify.Equal(requestTimes.Modify) {
				t.Errorf("modify: got %s, want %s", got.Modify, requestTimes.Modify)
			}
			switch tt.wantCreation {
			case "access":
				assertBetween(t, "access", got.Access, start, end)
			case "modify":
				assertBetween(t, "modify", got.Modify, start, end)
			}
		})
	}
}

func TestUpdateInvalidTimesDoesNotCreate(t *testing.T) {
	far := time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		times filetime.Times
		opts  Options
	}{
		{"zero", filetime.Times{}, Options{}},
		{"far access", filetime.Times{Access: far, Modify: requestTimes.Modify}, Options{Access: true}},
		{"far modify", filetime.Times{Access: requestTimes.Access, Modify: far}, Options{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "new")
			if err := Update(path, tt.times, tt.opts); err == nil {
				t.Fatal("want an error")
			}
			assertMissing(t, path)
		})
	}
}

func TestUpdateNoCreate(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing")
		if err := Update(path, requestTimes, Options{NoCreate: true}); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, path)
	})

	t.Run("existing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		writeSeeded(t, path, "")
		if err := Update(path, requestTimes, Options{NoCreate: true}); err != nil {
			t.Fatal(err)
		}
		if got := readTimes(t, path); !sameTimes(got, requestTimes) {
			t.Errorf("got %+v, want %+v", got, requestTimes)
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("directory search permission is not modeled by mode bits on Windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("permission checks do not apply to root")
		}
		dir := filepath.Join(t.TempDir(), "locked")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		if err := Update(filepath.Join(dir, "missing"), requestTimes, Options{NoCreate: true}); err == nil {
			t.Fatal("want a permission error")
		}
	})
}

func TestUpdateNoCreatePathPrefixes(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeSeeded(t, filepath.Join(root, "regular"), "")
	t.Chdir(root)

	tests := []struct {
		path    string
		wantErr bool
	}{
		{filepath.Join("missing", "child"), false},
		{filepath.Join("directory", "child"), false},
		{filepath.Join("directory", "missing", "child"), false},
		{filepath.Join("regular", "child"), true},
		{filepath.Join("regular", "missing", "child"), true},
		{filepath.Join(root, "regular", "child"), true},
		{filepath.Join(root, "directory", "child"), false},
		{"regular" + string(filepath.Separator) + string(filepath.Separator) + "child", true},
		{filepath.Join(".", "regular") + string(filepath.Separator) + "." + string(filepath.Separator) + "child", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := Update(tt.path, requestTimes, Options{NoCreate: true})
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("got error %v, want error %t", err, tt.wantErr)
			}
			assertMissing(t, tt.path)
		})
	}

	t.Run("without no-create", func(t *testing.T) {
		for _, path := range []string{filepath.Join("missing", "child"), filepath.Join("regular", "child")} {
			if err := Update(path, requestTimes, Options{}); err == nil {
				t.Errorf("%s: want an error", path)
			}
			assertMissing(t, path)
		}
	})
}

func TestUpdateLinks(t *testing.T) {
	t.Run("target", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		writeSeeded(t, target, "content")
		link := filepath.Join(dir, "link")
		symlink(t, target, link)

		if err := Update(link, requestTimes, Options{}); err != nil {
			t.Fatal(err)
		}
		if got := readTimes(t, target); !sameTimes(got, requestTimes) {
			t.Errorf("got %+v, want %+v", got, requestTimes)
		}
	})

	t.Run("dangling", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		link := filepath.Join(dir, "link")
		symlink(t, target, link)

		if err := Update(link, requestTimes, Options{}); err != nil {
			t.Fatal(err)
		}
		if got := readTimes(t, target); !sameTimes(got, requestTimes) {
			t.Errorf("got %+v, want %+v", got, requestTimes)
		}
	})

	t.Run("dangling with missing parent", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "missing", "target")
		link := filepath.Join(dir, "link")
		symlink(t, target, link)

		if err := Update(link, requestTimes, Options{}); err == nil {
			t.Fatal("want an error")
		}
		assertMissing(t, filepath.Join(dir, "missing"))
	})

	t.Run("dangling no-create", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		link := filepath.Join(dir, "link")
		symlink(t, target, link)

		if err := Update(link, requestTimes, Options{NoCreate: true}); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, target)
	})

	t.Run("parent link to regular file", func(t *testing.T) {
		dir := t.TempDir()
		regular := filepath.Join(dir, "regular")
		writeSeeded(t, regular, "")
		link := filepath.Join(dir, "link")
		symlink(t, regular, link)

		if err := Update(filepath.Join(link, "child"), requestTimes, Options{NoCreate: true}); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("dangling parent link", func(t *testing.T) {
		dir := t.TempDir()
		link := filepath.Join(dir, "link")
		symlink(t, filepath.Join(dir, "missing"), link)

		if err := Update(filepath.Join(link, "child"), requestTimes, Options{NoCreate: true}); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, filepath.Join(dir, "missing"))
	})

	t.Run("dot-dot after link", func(t *testing.T) {
		root := t.TempDir()
		nested := filepath.Join(root, "a", "b")
		if err := os.MkdirAll(nested, 0o700); err != nil {
			t.Fatal(err)
		}
		writeSeeded(t, filepath.Join(root, "file"), "")
		symlink(t, nested, filepath.Join(root, "link"))
		path := filepath.Join(root, "link") + string(filepath.Separator) + ".." +
			string(filepath.Separator) + "file" + string(filepath.Separator) + "child"

		err := Update(path, requestTimes, Options{NoCreate: true})
		lexical := runtime.GOOS == "windows"
		if gotErr := err != nil; gotErr != lexical {
			t.Fatalf("got error %v, want error %t", err, lexical)
		}
	})
}
