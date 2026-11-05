package filetime

import (
	"io/fs"
	"syscall"
	"testing"
	"time"
)

type rawInfo struct {
	t    *testing.T
	data *syscall.Win32FileAttributeData
}

func (rawInfo) Name() string      { return "raw" }
func (rawInfo) Size() int64       { return 0 }
func (rawInfo) Mode() fs.FileMode { return 0 }
func (rawInfo) IsDir() bool       { return false }
func (i rawInfo) Sys() any        { return i.data }
func (i rawInfo) ModTime() time.Time {
	i.t.Helper()
	i.t.Fatal("ModTime must not be used to decode Windows timestamps")
	return time.Time{}
}

func ticks(n uint64) syscall.Filetime {
	return syscall.Filetime{LowDateTime: uint32(n), HighDateTime: uint32(n >> 32)}
}

const (
	testSecondsPerDay  = 86400
	testTicksPerSecond = 10_000_000
	days1601To1970     = 134774
	days1601To2500     = 328353
)

func TestReadRawWindowsTimes(t *testing.T) {
	tests := []struct {
		name   string
		access syscall.Filetime
		modify syscall.Filetime
		want   Times
	}{
		{
			name:   "epochs",
			access: ticks(0),
			modify: ticks(days1601To1970 * testSecondsPerDay * testTicksPerSecond),
			want: Times{
				Access: time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC),
				Modify: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:   "fraction",
			access: ticks(days1601To1970*testSecondsPerDay*testTicksPerSecond + 1),
			modify: ticks(days1601To1970*testSecondsPerDay*testTicksPerSecond + 2*testTicksPerSecond + 3),
			want: Times{
				Access: time.Date(1970, 1, 1, 0, 0, 0, 100, time.UTC),
				Modify: time.Date(1970, 1, 1, 0, 0, 2, 300, time.UTC),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timesFromInfo(rawInfo{t, &syscall.Win32FileAttributeData{
				LastAccessTime: tt.access,
				LastWriteTime:  tt.modify,
			}})
			if err != nil {
				t.Fatal(err)
			}
			if !got.Access.Equal(tt.want.Access) || !got.Modify.Equal(tt.want.Modify) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadWindowsRange(t *testing.T) {
	ordinary := ticks(days1601To1970 * testSecondsPerDay * testTicksPerSecond)
	far := ticks(days1601To2500 * testSecondsPerDay * testTicksPerSecond)
	wantOrdinary := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	wantFar := time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		access syscall.Filetime
		modify syscall.Filetime
		want   Times
	}{
		{"far modify", ordinary, far, Times{Access: wantOrdinary, Modify: wantFar}},
		{"far access", far, ordinary, Times{Access: wantFar, Modify: wantOrdinary}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timesFromInfo(rawInfo{t, &syscall.Win32FileAttributeData{
				LastAccessTime: tt.access,
				LastWriteTime:  tt.modify,
			}})
			if err != nil {
				t.Fatal(err)
			}
			if !got.Access.Equal(tt.want.Access) || !got.Modify.Equal(tt.want.Modify) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadWindowsUnexpectedSys(t *testing.T) {
	if _, err := timesFromInfo(rawInfo{t, nil}); err == nil {
		t.Fatal("want an error for missing Windows metadata")
	}
}
