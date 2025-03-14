package logger_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mxmauro/logger"
	"github.com/mxmauro/logger/engines/file"
)

//------------------------------------------------------------------------------

func TestFileLog(t *testing.T) {
	if dir, err := filepath.Abs(filepath.FromSlash("./testdata/logs")); err == nil {
		_ = os.RemoveAll(dir)
	}

	lg := logger.Create(logger.Options{
		Level:      logger.LogLevelDebug,
		DebugLevel: 1,
	})
	defer lg.Destroy()

	err := lg.AddFileEngine(file.Options{
		Prefix:     "Test",
		Directory:  "./testdata/logs",
		DaysToKeep: 7,
	})
	if err != nil {
		t.Errorf("unable to initialize. [%v]", err)
		return
	}

	printTestMessages(lg)
}

func TestFileLogWithVaultLimit(t *testing.T) {
	if dir, err := filepath.Abs(filepath.FromSlash("./testdata/logs")); err == nil {
		_ = os.RemoveAll(dir)
	}

	lg := logger.Create(logger.Options{
		Level:      logger.LogLevelDebug,
		DebugLevel: 1,
	})
	defer lg.Destroy()

	err := lg.AddFileEngine(file.Options{
		Prefix:           "Test",
		Directory:        "./testdata/logs",
		DaysToKeep:       7,
		MaxFileSize:      65536,
		MaxFileVaultSize: 200 * 1024, //200Kb
	})
	if err != nil {
		t.Errorf("unable to initialize. [%v]", err)
		return
	}

	// 2500 times should be enough
	for i := 1; i <= 2500; i++ {
		printTestMessages(lg)
	}
}
