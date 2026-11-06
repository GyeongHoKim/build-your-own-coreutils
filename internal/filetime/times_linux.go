package filetime

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func timesFromInfo(info os.FileInfo) (Times, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return Times{}, fmt.Errorf("unexpected metadata type %T", info.Sys())
	}
	return Times{
		Access: time.Unix(stat.Atim.Unix()),
		Modify: time.Unix(stat.Mtim.Unix()),
	}, nil
}
