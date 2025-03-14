package file

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mxmauro/logger/engines"
)

//------------------------------------------------------------------------------

const (
	minFileSize      = 10 * 1024
	minFileVaultSize = 100 * 1024
)

//------------------------------------------------------------------------------

// Options specifies the file logger settings to use when it is created.
type Options struct {
	// Filename prefix to use when a file is created. Defaults to the binary name.
	Prefix string `json:"prefix,omitempty"`

	// Destination directory to store log files.
	Directory string `json:"dir,omitempty"`

	// Amount of days to keep old logs.
	DaysToKeep uint `json:"daysToKeep,omitempty"`

	// Set the maximum file size. Minimum is 10Kb. Unlimited if zero.
	MaxFileSize uint64 `json:"maxFileSize,omitempty"`

	// Set the maximum file storage size. Minimum is 1Mb. Unlimited if zero.
	MaxFileVaultSize uint64 `json:"maxFileVaultSize,omitempty"`
}

type engine struct {
	mtx                  sync.Mutex
	fd                   *os.File
	lastWasError         int32
	directory            string
	daysToKeep           uint
	maxFileSize          int64
	maxFileVaultSize     int64
	prefix               string
	subFileIndex         int
	dayOfFile            int
	currentFileSize      int64
	currentFileVaultSize int64
}

//------------------------------------------------------------------------------

func NewEngine(opts Options) (engines.Engine, error) {
	var err error

	if len(opts.Prefix) == 0 {
		// If no prefix was given, use the base name of the executable.
		opts.Prefix, err = os.Executable()
		if err != nil {
			return nil, err
		}
		opts.Prefix = filepath.Base(opts.Prefix)

		extLen := len(filepath.Ext(opts.Prefix))
		if len(opts.Prefix) > extLen {
			opts.Prefix = opts.Prefix[:(len(opts.Prefix) - extLen)]
		}
	}

	// Create file adapter
	lg := &engine{
		prefix:    opts.Prefix,
		dayOfFile: -1,
	}

	// Set the number of days to keep the old files
	if opts.DaysToKeep < 365 {
		lg.daysToKeep = opts.DaysToKeep
	} else {
		lg.daysToKeep = 365
	}

	// Establishes the target directory
	if len(opts.Directory) > 0 {
		lg.directory = filepath.ToSlash(opts.Directory)
	} else {
		lg.directory = "logs"
	}

	if !filepath.IsAbs(lg.directory) {
		var workingDir string

		workingDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}

		lg.directory = filepath.Join(workingDir, lg.directory)
	}
	lg.directory = filepath.Clean(lg.directory)
	if !strings.HasSuffix(lg.directory, string(filepath.Separator)) {
		lg.directory += string(filepath.Separator)
	}

	// File size and vault limits
	if opts.MaxFileSize > 0 {
		if opts.MaxFileSize > uint64(math.MaxInt64) {
			lg.maxFileSize = int64(math.MaxInt64)
		} else if opts.MaxFileSize < uint64(minFileSize) {
			lg.maxFileSize = int64(minFileSize)
		} else {
			lg.maxFileSize = int64(opts.MaxFileSize)
		}
	}

	if opts.MaxFileVaultSize > 0 {
		if opts.MaxFileVaultSize > uint64(math.MaxInt64) {
			lg.maxFileVaultSize = int64(math.MaxInt64)
		} else if opts.MaxFileVaultSize < uint64(minFileVaultSize) {
			lg.maxFileVaultSize = int64(minFileVaultSize)
		} else {
			lg.maxFileVaultSize = int64(opts.MaxFileVaultSize)
		}
		if lg.maxFileVaultSize < lg.maxFileSize {
			lg.maxFileVaultSize = lg.maxFileSize
		}
	}

	// Delete old files and get the current vault size
	lg.currentFileVaultSize, _ = lg.purgeFileVault()

	// Done
	return lg, nil
}

func (lg *engine) Class() string {
	return "file"
}

func (lg *engine) Destroy() {
	lg.mtx.Lock()
	defer lg.mtx.Unlock()

	if lg.fd != nil {
		_ = lg.fd.Sync()
		_ = lg.fd.Close()
		lg.fd = nil
	}
}

func (lg *engine) Success(now time.Time, msg string, raw bool, _ bool) {
	if !raw {
		lg.write(now, "SUCCESS", msg)
	} else {
		lg.writeRAW(now, msg)
	}
}

func (lg *engine) Error(now time.Time, msg string, raw bool) {
	if !raw {
		lg.write(now, "ERROR", msg)
	} else {
		lg.writeRAW(now, msg)
	}
}

func (lg *engine) Warning(now time.Time, msg string, raw bool) {
	if !raw {
		lg.write(now, "WARNING", msg)
	} else {
		lg.writeRAW(now, msg)
	}
}

func (lg *engine) Info(now time.Time, msg string, raw bool) {
	if !raw {
		lg.write(now, "INFO", msg)
	} else {
		lg.writeRAW(now, msg)
	}
}

func (lg *engine) Debug(now time.Time, msg string, raw bool) {
	if !raw {
		lg.write(now, "DEBUG", msg)
	} else {
		lg.writeRAW(now, msg)
	}
}

func (lg *engine) write(now time.Time, level string, msg string) {
	sb := strings.Builder{}
	_, _ = sb.WriteString(now.Format("2006-01-02 15:04:05.000"))
	_, _ = sb.WriteString(" [")
	_, _ = sb.WriteString(level)
	_, _ = sb.WriteString("]: ")
	_, _ = sb.WriteString(msg)
	lg.writeRAW(now, sb.String())
}

func (lg *engine) writeRAW(now time.Time, msg string) {
	msgLen := len(msg)

	// Lock access
	lg.mtx.Lock()
	defer lg.mtx.Unlock()

	err := lg.openOrRotateFile(now, msgLen+newLineLen)
	if err == nil {
		// Save message to file
		_, err = lg.fd.WriteString(msg)
		if err == nil {
			lg.currentFileSize += int64(msgLen)
			lg.currentFileVaultSize += int64(msgLen)
			_, err = lg.fd.WriteString(newLine)
			if err == nil {
				lg.currentFileSize += int64(newLineLen)
				lg.currentFileVaultSize += int64(newLineLen)
			}
		}
	}
}

func (lg *engine) openOrRotateFile(now time.Time, msgLen int) error {
	dayOfNow := now.Day()

	// Check if we have to rotate files
	rotate := lg.fd == nil || dayOfNow != lg.dayOfFile ||
		(lg.maxFileSize > 0 && lg.currentFileSize+int64(msgLen) > lg.maxFileSize) ||
		(lg.maxFileVaultSize > 0 && lg.currentFileVaultSize+int64(msgLen) > lg.maxFileVaultSize)
	if !rotate {
		return nil
	}

	// Close old file if anyone is open
	if lg.fd != nil {
		_ = lg.fd.Sync()
		_ = lg.fd.Close()
		lg.fd = nil
	}
	lg.currentFileSize = 0
	if lg.maxFileSize > 0 {
		if dayOfNow != lg.dayOfFile {
			lg.subFileIndex = 1
		} else {
			lg.subFileIndex += 1
		}
	}

	// Delete old files and get the current vault size
	lg.currentFileVaultSize, _ = lg.purgeFileVault()

	// Create target directory if it does not exist
	err := os.MkdirAll(lg.directory, 0755)
	if err != nil {
		return err
	}

	// Create a new log file
	filenameSB := strings.Builder{}
	_, _ = filenameSB.WriteString(lg.directory)
	_, _ = filenameSB.WriteString(strings.ToLower(lg.prefix))
	_, _ = filenameSB.WriteString(".")
	_, _ = filenameSB.WriteString(now.Format("2006-01-02"))
	if lg.maxFileSize > 0 {
		_, _ = filenameSB.WriteString("-")
		_, _ = filenameSB.WriteString(fmt.Sprintf("%03d", lg.subFileIndex))
	}
	_, _ = filenameSB.WriteString(".log")

	lg.fd, err = os.OpenFile(filenameSB.String(), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	lg.dayOfFile = dayOfNow

	// Done
	return nil
}

// This also returns the current vault size
func (lg *engine) purgeFileVault() (int64, error) {
	type LogFile struct {
		Name      string
		FileSize  int64
		CreatedAt time.Time
	}

	if lg.daysToKeep == 0 && lg.maxFileVaultSize == 0 {
		return 0, nil // Nothing to do
	}

	// Get all log files
	files, err := os.ReadDir(lg.directory)
	if err != nil {
		return 0, err
	}

	// Filter undesired files
	filteredFiles := make([]LogFile, 0, len(files))
	for _, f := range files {
		var fi fs.FileInfo

		if f.IsDir() {
			continue // Ignore directories
		}

		filename := f.Name()
		filenameLen := len(filename)
		if filenameLen < 4 || strings.ToLower(filename[filenameLen-4:]) != ".log" {
			continue // Ignore non-log files
		}

		fi, err = f.Info()
		if err != nil {
			continue
		}

		filteredFiles = append(filteredFiles, LogFile{
			Name:      filename,
			FileSize:  fi.Size(),
			CreatedAt: getFileCreationTime(fi),
		})
	}
	filteredFilesLen := len(filteredFiles)

	// Sort the list by the file creation date
	slices.SortFunc(filteredFiles, func(a, b LogFile) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	// Find the cut point for old files
	deleteUntilIndex := 0
	if lg.daysToKeep > 0 {
		lowestTime := time.Now().UTC().AddDate(0, 0, -(int(lg.daysToKeep)))
		for deleteUntilIndex = 0; deleteUntilIndex < filteredFilesLen; deleteUntilIndex += 1 {
			if !filteredFiles[deleteUntilIndex].CreatedAt.Before(lowestTime) {
				break
			}
		}
	}

	// Calculate the size of the remaining files
	fileVaultSize := int64(0)
	for idx := deleteUntilIndex; idx < filteredFilesLen; idx++ {
		fileVaultSize += filteredFiles[idx].FileSize
	}

	// Check if we need more space
	if lg.maxFileVaultSize > 0 {
		requiredMaxSize := lg.maxFileVaultSize - minFileSize
		for deleteUntilIndex < filteredFilesLen && fileVaultSize > requiredMaxSize {
			fileVaultSize -= filteredFiles[deleteUntilIndex].FileSize
			deleteUntilIndex += 1
		}
	}

	// Delete the files we dont need
	for idx := 0; idx < deleteUntilIndex; idx++ {
		_ = os.Remove(lg.directory + filteredFiles[idx].Name)
	}

	// Done
	return fileVaultSize, nil
}
