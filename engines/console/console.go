package console

import (
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/muesli/termenv"
	"github.com/mxmauro/logger/engines"
)

//------------------------------------------------------------------------------

// Options specifies the console logger settings to use when it is created.
type Options struct {
	// Do not print colored output.
	DisableColor bool `json:"disableColor,omitempty"`
}

type engine struct {
	themedLevels [5]string
}

//------------------------------------------------------------------------------

func NewEngine(opts Options) engines.Engine {
	// Create console adapter
	lg := &engine{}

	if opts.DisableColor || termenv.ColorProfile() == termenv.Ascii {
		lg.themedLevels[0] = "[ERROR]"
		lg.themedLevels[1] = "[WARN]"
		lg.themedLevels[2] = "[INFO]"
		lg.themedLevels[3] = "[DEBUG]"
		lg.themedLevels[4] = "[SUCCESS]"
	} else {
		lg.themedLevels[0] = color.New(color.BlinkRapid, color.FgHiWhite, color.BgRed).Sprintf("[ERROR]")
		lg.themedLevels[1] = color.New(color.FgHiYellow).Sprintf("[WARN]")
		lg.themedLevels[2] = color.New(color.FgHiBlue).Sprintf("[INFO]")
		lg.themedLevels[3] = color.New(color.FgCyan).Sprintf("[DEBUG]")
		lg.themedLevels[3] = color.New(color.FgHiGreen).Sprintf("[SUCCESS]")
	}

	// Done
	return lg
}

func (lg *engine) Class() string {
	return "console"
}

func (lg *engine) Destroy() {
	// Do nothing
}

func (lg *engine) Success(now time.Time, msg string, raw bool, sendSuccessAtErrorLogLevel bool) {
	of := os.Stdout
	if sendSuccessAtErrorLogLevel {
		of = os.Stderr
	}
	if !raw {
		consolePrint(of, now, lg.themedLevels[4], msg)
	} else {
		consolePrintRAW(of, msg)
	}
}

func (lg *engine) Error(now time.Time, msg string, raw bool) {
	if !raw {
		consolePrint(os.Stderr, now, lg.themedLevels[0], msg)
	} else {
		consolePrintRAW(os.Stderr, msg)
	}
}

func (lg *engine) Warning(now time.Time, msg string, raw bool) {
	if !raw {
		consolePrint(os.Stderr, now, lg.themedLevels[1], msg)
	} else {
		consolePrintRAW(os.Stderr, msg)
	}
}

func (lg *engine) Info(now time.Time, msg string, raw bool) {
	if !raw {
		consolePrint(os.Stdout, now, lg.themedLevels[2], msg)
	} else {
		consolePrintRAW(os.Stdout, msg)
	}
}

func (lg *engine) Debug(now time.Time, msg string, raw bool) {
	if !raw {
		consolePrint(os.Stdout, now, lg.themedLevels[3], msg)
	} else {
		consolePrintRAW(os.Stdout, msg)
	}
}
