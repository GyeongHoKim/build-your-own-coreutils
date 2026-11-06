package main

import (
	"os"
	"syscall"
	"testing"
	"time"
)

const (
	windowsTicksPerSecond = 10_000_000
	windowsEpochOffset    = 11644473600
)

func ticksOf(t time.Time) syscall.Filetime {
	n := uint64(t.Unix()+windowsEpochOffset) * windowsTicksPerSecond
	return syscall.Filetime{LowDateTime: uint32(n), HighDateTime: uint32(n >> 32)}
}

func setRawTimes(t *testing.T, path string, access, modify syscall.Filetime) bool {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(name, syscall.FILE_WRITE_ATTRIBUTES|syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.CloseHandle(h); err != nil {
			t.Error(err)
		}
	}()
	if err := syscall.SetFileTime(h, nil, &access, &modify); err != nil {
		t.Skipf("cannot store raw timestamps: %v", err)
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		t.Fatal(err)
	}
	return info.LastAccessTime == access && info.LastWriteTime == modify
}

func TestRunReferenceRange(t *testing.T) {
	ordinary := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	far := time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		refAccess  time.Time
		refModify  time.Time
		args       []string
		status     int
		wantAccess time.Time
		wantModify time.Time
	}{
		{"far access default", far, ordinary, nil, 1, seedAccess, seedModify},
		{"far access default -c", far, ordinary, []string{"-c"}, 1, seedAccess, seedModify},
		{"far access selected", far, ordinary, []string{"-a"}, 1, seedAccess, seedModify},
		{"far access unselected", far, ordinary, []string{"-m"}, 0, seedAccess, ordinary},
		{"far modify default", ordinary, far, nil, 1, seedAccess, seedModify},
		{"far modify selected -c", ordinary, far, []string{"-c", "-m"}, 1, seedAccess, seedModify},
		{"far modify unselected", ordinary, far, []string{"-a"}, 0, ordinary, seedModify},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTempDir(t)
			if err := os.WriteFile("ref", nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if !setRawTimes(t, "ref", ticksOf(tt.refAccess), ticksOf(tt.refModify)) {
				t.Skip("the filesystem does not store the requested reference timestamps")
			}
			writeSeeded(t, "existing")

			args := append(append([]string{}, tt.args...), "-r", "ref", "existing", "new")
			got := invoke(t, args...)
			if tt.status != 0 {
				assertFailure(t, got, tt.status)
				assertMissing(t, "new")
			} else {
				assertSuccess(t, got)
			}
			assertTimes(t, "existing", tt.wantAccess, tt.wantModify)
		})
	}
}
