package logger

import (
	"errors"
	"sync"

	"github.com/mxmauro/logger/engines"
	"github.com/mxmauro/logger/engines/console"
	"github.com/mxmauro/logger/engines/file"
	"github.com/mxmauro/logger/engines/syslog"
)

//------------------------------------------------------------------------------

// Logger is the object that controls logging.
type Logger struct {
	mtx                        sync.RWMutex
	engines                    []engines.Engine
	logLevel                   LogLevel
	debugLogLevel              uint
	useLocalTime               bool
	sendSuccessAtErrorLogLevel bool
}

// Options specifies the logger settings to use when initialized.
type Options struct {
	// Set the initial logging level to use.
	Level LogLevel `json:"level,omitempty"`

	// Set the initial logging level for debug output to use.
	DebugLevel uint `json:"debugLevel,omitempty"`

	// Use the local computer time instead of UTC.
	UseLocalTime bool `json:"useLocalTime,omitempty"`

	// By default, success messages are sent at "Info" log level but you can change it
	// to send them along with error messages.
	SendSuccessAtErrorLogLevel bool `json:"successAtErrorLogLevel,omitempty"`
}

// LogLevel defines the level of message verbosity.
type LogLevel uint

// -----------------------------------------------------------------------------

const (
	LogLevelQuiet   LogLevel = 0
	LogLevelError   LogLevel = 1
	LogLevelWarning LogLevel = 2
	LogLevelInfo    LogLevel = 3
	LogLevelDebug   LogLevel = 4
)

//------------------------------------------------------------------------------

var (
	defaultLoggerInit = sync.Once{}
	defaultLogger     *Logger
)

//------------------------------------------------------------------------------

// Default returns a logger that only outputs error and warnings to the console.
func Default() *Logger {
	defaultLoggerInit.Do(func() {
		defaultLogger = Create(Options{
			Level: LogLevelInfo,
		})
		defaultLogger.AddConsoleEngine(console.Options{})
	})
	return defaultLogger
}

// Create creates a new logger.
func Create(opts Options) *Logger {
	// Create logger
	lg := &Logger{
		mtx:                        sync.RWMutex{},
		engines:                    make([]engines.Engine, 0),
		logLevel:                   opts.Level,
		debugLogLevel:              opts.DebugLevel,
		useLocalTime:               opts.UseLocalTime,
		sendSuccessAtErrorLogLevel: opts.SendSuccessAtErrorLogLevel,
	}

	// Done
	return lg
}

// Destroy shuts down the logger.
func (lg *Logger) Destroy() {
	// Lock access
	lg.mtx.Lock()
	defer lg.mtx.Unlock()

	// The default logger cannot be destroyed
	if lg == defaultLogger {
		return
	}

	// Destroy all engines
	for _, engine := range lg.engines {
		engine.Destroy()
	}
	lg.engines = nil
}

// AddConsoleEngine adds a console output to the logger.
func (lg *Logger) AddConsoleEngine(opts console.Options) {
	engine := console.NewEngine(opts)
	_ = lg.AddEngine(engine)
}

// AddFileEngine adds a file-based output to the logger.
func (lg *Logger) AddFileEngine(opts file.Options) error {
	engine, err := file.NewEngine(opts)
	if err != nil {
		return err
	}
	return lg.AddEngine(engine)
}

// AddSysLogEngine adds the engine that sends the output to SysLog compatible servers.
func (lg *Logger) AddSysLogEngine(opts syslog.Options) error {
	engine, err := syslog.NewEngine(opts)
	if err != nil {
		return err
	}
	return lg.AddEngine(engine)
}

func (lg *Logger) AddEngine(engine engines.Engine) error {
	if engine == nil {
		return errors.New("invalid engine")
	}

	// Lock access
	lg.mtx.Lock()
	defer lg.mtx.Unlock()

	// Add engine
	lg.engines = append(lg.engines, engine)

	// Done
	return nil
}

// SetLogLevel sets the minimum level for all messages.
func (lg *Logger) SetLogLevel(level LogLevel, debugLevel uint) {
	// Lock access
	lg.mtx.Lock()
	defer lg.mtx.Unlock()

	lg.logLevel = level
	lg.debugLogLevel = debugLevel
}

// Success emits a success message into the configured targets.
// If a string is passed, output format will be in DATE [LEVEL] MESSAGE.
// If a struct is passed, output will be in json with level and timestamp fields automatically added.
func (lg *Logger) Success(obj interface{}) {
	// Lock access
	lg.mtx.RLock()
	defer lg.mtx.RUnlock()

	minLogLevel := LogLevelInfo
	if lg.sendSuccessAtErrorLogLevel {
		minLogLevel = LogLevelError
	}
	if lg.logLevel < minLogLevel {
		return
	}

	lg.log(obj, "success", logTypeSuccess)
}

// Error emits an error message into the configured targets.
// If a string is passed, output format will be in DATE [LEVEL] MESSAGE.
// If a struct is passed, output will be in json with level and timestamp fields automatically added.
func (lg *Logger) Error(obj interface{}) {
	// Lock access
	lg.mtx.RLock()
	defer lg.mtx.RUnlock()

	if lg.logLevel < LogLevelError {
		return
	}

	lg.log(obj, "error", logTypeError)
}

// Warning emits a warning message into the configured targets.
// If a string is passed, output format will be in DATE [LEVEL] MESSAGE.
// If a struct is passed, output will be in json with level and timestamp fields automatically added.
func (lg *Logger) Warning(obj interface{}) {
	// Lock access
	lg.mtx.RLock()
	defer lg.mtx.RUnlock()

	if lg.logLevel < LogLevelWarning {
		return
	}

	lg.log(obj, "warning", logTypeWarning)
}

// Info emits an information message into the configured targets.
// If a string is passed, output format will be in DATE [LEVEL] MESSAGE.
// If a struct is passed, output will be in json with level and timestamp fields automatically added.
func (lg *Logger) Info(obj interface{}) {
	// Lock access
	lg.mtx.RLock()
	defer lg.mtx.RUnlock()

	if lg.logLevel < LogLevelInfo {
		return
	}

	lg.log(obj, "info", logTypeInfo)
}

// Debug emits a debug message into the configured targets.
// If a string is passed, output format will be in DATE [LEVEL] MESSAGE.
// If a struct is passed, output will be in json with level and timestamp fields automatically added.
func (lg *Logger) Debug(level uint, obj interface{}) {
	// Lock access
	lg.mtx.RLock()
	defer lg.mtx.RUnlock()

	if lg.logLevel < LogLevelDebug || lg.debugLogLevel < level {
		return
	}

	lg.log(obj, "debug", logTypeDebug)
}
