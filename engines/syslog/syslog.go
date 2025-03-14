package syslog

import (
	"container/list"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mxmauro/logger/engines"
	"github.com/mxmauro/resetevent"
)

//------------------------------------------------------------------------------

const (
	severityError         = 3
	severityWarning       = 4
	severityInformational = 6
	severityDebug         = 7

	facilityUser = 1

	defaultMaxMessageQueueSize = 1024

	flushTimeout = 5 * time.Second
)

//------------------------------------------------------------------------------

// Options specifies the syslog settings to use when it is created.
type Options struct {
	// Application name to use. Defaults to the binary name.
	AppName string `json:"appName,omitempty"`

	// Syslog server host name.
	Host string `json:"host,omitempty"`

	// Syslog server port. Defaults to 514, 1468 or 6514 depending on the network protocol used.
	Port uint16 `json:"port,omitempty"`

	// Use TCP instead of UDP.
	UseTcp bool `json:"useTcp,omitempty"`

	// Uses a secure connection. Implies TCP.
	UseTls bool `json:"useTls,omitempty"`

	// Send messages in the new RFC 5424 format instead of the original RFC 3164 specification.
	UseRFC5424 bool `json:"useRFC5424,omitempty"`

	// Set the maximum amount of messages to keep in memory if connection to the server is lost.
	MaxMessageQueueSize uint `json:"queueSize,omitempty"`

	// TLSConfig optionally provides a TLS configuration for use.
	TlsConfig *tls.Config
}

type engine struct {
	conn            net.Conn
	appName         string
	serverAddress   string
	useTcp          bool
	tlsConfig       *tls.Config
	useRFC5424      bool
	hostname        string
	pid             int
	mtx             sync.Mutex
	queue           *list.List
	queueAvailEv    *resetevent.AutoResetEvent
	maxQueueSize    uint
	shutdownOnce    sync.Once
	wg              sync.WaitGroup
	workerCtx       context.Context
	workerCancelCtx context.CancelFunc
}

//------------------------------------------------------------------------------

func NewEngine(opts Options) (engines.Engine, error) {
	if len(opts.AppName) == 0 {
		var err error

		// If no application name was given, use the base name of the executable.
		opts.AppName, err = os.Executable()
		if err != nil {
			return nil, err
		}
		opts.AppName = filepath.Base(opts.AppName)

		extLen := len(filepath.Ext(opts.AppName))
		if len(opts.AppName) > extLen {
			opts.AppName = opts.AppName[:(len(opts.AppName) - extLen)]
		}
	}

	// Create Syslog adapter
	lg := &engine{
		appName:      opts.AppName,
		useTcp:       opts.UseTcp,
		useRFC5424:   opts.UseRFC5424,
		pid:          os.Getpid(),
		mtx:          sync.Mutex{},
		queue:        list.New(),
		queueAvailEv: resetevent.NewAutoResetEvent(),
		maxQueueSize: opts.MaxMessageQueueSize,
		shutdownOnce: sync.Once{},
		wg:           sync.WaitGroup{},
	}
	if opts.MaxMessageQueueSize == 0 {
		lg.maxQueueSize = defaultMaxMessageQueueSize
	}

	lg.workerCtx, lg.workerCancelCtx = context.WithCancel(context.Background())

	if opts.UseTls {
		if opts.TlsConfig != nil {
			lg.tlsConfig = opts.TlsConfig.Clone()
		} else {
			lg.tlsConfig = &tls.Config{
				MinVersion: 2,
			}
		}
	}

	// Set the server host
	if len(opts.Host) > 0 {
		lg.serverAddress = opts.Host
	} else {
		lg.serverAddress = "127.0.0.1"
	}

	// Set the server port
	port := opts.Port
	if opts.Port == 0 {
		if opts.UseTcp {
			if opts.UseTls {
				port = 6514
			} else {
				port = 1468
			}
		} else {
			port = 514
		}
	}
	lg.serverAddress += ":" + strconv.Itoa(int(port))

	// Set the client host name
	lg.hostname, _ = os.Hostname()

	// Create a background messenger worker
	lg.wg.Add(1)
	go lg.messengerWorker()

	// Done
	return lg, nil
}

func (lg *engine) Class() string {
	return "syslog"
}

func (lg *engine) Destroy() {
	lg.shutdownOnce.Do(func() {
		// Stop worker
		lg.workerCancelCtx()

		// Wait until exits
		lg.wg.Wait()

		lg.workerCtx = nil
		lg.workerCancelCtx = nil

		// Flush queued messages
		lg.flushQueue()

		// Disconnect from the network
		lg.disconnect()
	})
}

func (lg *engine) Success(now time.Time, msg string, raw bool, sendSuccessAtErrorLogLevel bool) {
	if sendSuccessAtErrorLogLevel {
		lg.writeString(facilityUser, severityError, now, msg, raw)
	} else {
		lg.writeString(facilityUser, severityInformational, now, msg, raw)
	}
}

func (lg *engine) Error(now time.Time, msg string, raw bool) {
	lg.writeString(facilityUser, severityError, now, msg, raw)
}

func (lg *engine) Warning(now time.Time, msg string, raw bool) {
	lg.writeString(facilityUser, severityWarning, now, msg, raw)
}

func (lg *engine) Info(now time.Time, msg string, raw bool) {
	lg.writeString(facilityUser, severityInformational, now, msg, raw)
}

func (lg *engine) Debug(now time.Time, msg string, raw bool) {
	lg.writeString(facilityUser, severityDebug, now, msg, raw)
}

func (lg *engine) writeString(facility int, severity int, now time.Time, msg string, _ bool) {
	// Establish priority
	priority := (facility * 8) + severity

	// Remove or add new line depending on the transport protocol
	if lg.useTcp {
		if !strings.HasSuffix(msg, "\n") {
			msg = msg + "\n"
		}
	} else {
		msg = strings.TrimSuffix(msg, "\n")
	}

	// Format and queue the message
	// NOTE: We don't need to care here about the message type because level and timestamp are in separate fields.
	if !lg.useRFC5424 {
		lg.queueMessage("<" + strconv.Itoa(priority) + ">" + now.Format("Jan _2 15:04:05") + " " +
			lg.hostname + " " + msg)
	} else {
		lg.queueMessage("<" + strconv.Itoa(priority) + ">1 " + now.Format("2006-02-01T15:04:05Z") + " " +
			lg.hostname + " " + lg.appName + " " + strconv.Itoa(lg.pid) + " - - " + msg)
	}
}

func (lg *engine) queueMessage(msg string) {
	// Lock access
	lg.mtx.Lock()
	defer lg.mtx.Unlock()

	// Add to queue
	if uint(lg.queue.Len()) > lg.maxQueueSize {
		elem := lg.queue.Front()
		if elem != nil {
			lg.queue.Remove(elem)
		}
	}
	lg.queue.PushBack(msg)

	// Wake up worker if needed
	lg.queueAvailEv.Set()
}

func (lg *engine) dequeueMessage() (string, bool) {
	// Lock access
	lg.mtx.Lock()
	defer lg.mtx.Unlock()

	elem := lg.queue.Front()
	if elem == nil {
		return "", false
	}

	lg.queue.Remove(elem)
	return elem.Value.(string), true
}

// The messenger worker do actual message delivery. The intention of this goroutine, is to
// avoid halting the routine that sends the message if there are network issues.
func (lg *engine) messengerWorker() {
	defer lg.wg.Done()

	for {
		select {
		case <-lg.workerCtx.Done():
			return

		case <-lg.queueAvailEv.WaitCh():
			for {
				msg, ok := lg.dequeueMessage()
				if !ok {
					break
				}

				// Send message to server
				err := lg.writeBytes(lg.workerCtx, []byte(msg))

				// Handle error
				if err != nil && errors.Is(err, context.Canceled) {
					return
				}
			}
		}
	}
}

func (lg *engine) flushQueue() {
	ctx, cancelCtx := context.WithDeadline(context.Background(), time.Now().Add(flushTimeout))
	defer cancelCtx()

	for {
		// Dequeue next message
		elem := lg.queue.Front()
		if elem == nil {
			break // Reached the end
		}
		lg.queue.Remove(elem)

		// Send message to server
		err := lg.writeBytes(ctx, []byte(elem.Value.(string)))
		if err != nil {
			break // Stop on error
		}
	}
}

func (lg *engine) connect(ctx context.Context) error {
	var err error

	lg.disconnect()

	if lg.useTcp {
		if lg.tlsConfig != nil {
			dialer := tls.Dialer{
				Config: lg.tlsConfig,
			}
			lg.conn, err = dialer.DialContext(ctx, "tcp", lg.serverAddress)
		} else {
			dialer := net.Dialer{}
			lg.conn, err = dialer.DialContext(ctx, "tcp", lg.serverAddress)
		}
	} else {
		dialer := net.Dialer{}
		lg.conn, err = dialer.DialContext(ctx, "udp", lg.serverAddress)
	}

	return err
}

func (lg *engine) disconnect() {
	if lg.conn != nil {
		_ = lg.conn.Close()
		lg.conn = nil
	}
}

func (lg *engine) writeBytes(ctx context.Context, b []byte) error {
	// Send the message if connected
	if lg.conn != nil {
		_, err := lg.conn.Write(b)
		if err == nil {
			return nil
		}
	}

	// On error or if disconnected, try to connect
	err := lg.connect(ctx)
	if err == nil {
		_, err = lg.conn.Write(b)
		if err != nil {
			lg.disconnect()
		}
	}

	// Done
	return err
}
