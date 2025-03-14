package console

import (
	"fmt"
	"io"
	"sync"
	"time"
)

//------------------------------------------------------------------------------

var (
	consoleMtx = sync.Mutex{}
)

//------------------------------------------------------------------------------

func consolePrint(w io.Writer, now time.Time, themedLevel string, msg string) {
	// Lock console access
	consoleMtx.Lock()
	defer consoleMtx.Unlock()

	// Print the message prefixed with the timestamp and level
	_, _ = fmt.Fprintf(w, "%v %v %v\n", now.Format("2006-01-02 15:04:05.000"), themedLevel, msg)
}

func consolePrintRAW(w io.Writer, msg string) {
	// Lock console access
	consoleMtx.Lock()
	defer consoleMtx.Unlock()

	// Print the message with extra payload
	_, _ = fmt.Fprintf(w, "%v\n", msg)
}

