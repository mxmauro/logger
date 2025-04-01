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
	stat := fi.Sys().(*syscall.Dir)
	return time.Unix(int64(stat.Mtime), 0)
}
