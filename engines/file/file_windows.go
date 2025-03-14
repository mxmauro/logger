package file

import (
	"os"
	"syscall"
	"time"
)

//------------------------------------------------------------------------------

const (
	newLine = "\r\n"
	newLineLen = 2
)

//------------------------------------------------------------------------------

func getFileCreationTime(fi os.FileInfo) time.Time {
	stat := fi.Sys().(*syscall.Win32FileAttributeData)
	return time.Unix(0, stat.CreationTime.Nanoseconds())
}
