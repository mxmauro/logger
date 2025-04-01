package file

import (
	"os"
	"syscall"
	"time"
)

//------------------------------------------------------------------------------

const (
	newLine = "\n"
	newLineLen = 1
)

//------------------------------------------------------------------------------

func getFileCreationTime(fi os.FileInfo) time.Time {
	stat := fi.Sys().(*syscall.Stat_t)
	return time.Unix(stat.Ctime, stat.CtimeNsec)
}
