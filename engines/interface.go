package engines

import (
	"time"
)

// -----------------------------------------------------------------------------

type LogType uint

const (
	LogTypeSuccess LogType = iota
	LogTypeError
	LogTypeWarning
	LogTypeInfo
	LogTypeDebug
)

type Engine interface {
	Destroy()

	Success(now time.Time, msg string, raw bool, sendSuccessAtErrorLogLevel bool)
	Error(now time.Time, msg string, raw bool)
	Warning(now time.Time, msg string, raw bool)
	Info(now time.Time, msg string, raw bool)
	Debug(now time.Time, msg string, raw bool)
}
