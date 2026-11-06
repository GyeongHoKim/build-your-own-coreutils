package filetime

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

const (
	ticksPerSecond     = 10_000_000
	secondsPerHighUnit = (1 << 32) / ticksPerSecond
	ticksPerHighUnit   = (1 << 32) % ticksPerSecond
	secondsBeforeEpoch = 11644473600
)

func timesFromInfo(info os.FileInfo) (Times, error) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || data == nil {
		return Times{}, fmt.Errorf("unexpected metadata type %T", info.Sys())
	}
	return Times{
		Access: filetimeToTime(data.LastAccessTime),
		Modify: filetimeToTime(data.LastWriteTime),
	}, nil
}

func filetimeToTime(value syscall.Filetime) time.Time {
	high := int64(value.HighDateTime)
	low := int64(value.LowDateTime)
	ticks := high*ticksPerHighUnit + low
	seconds := high*secondsPerHighUnit + ticks/ticksPerSecond
	nanoseconds := ticks % ticksPerSecond * 100
	return time.Unix(seconds-secondsBeforeEpoch, nanoseconds)
}
