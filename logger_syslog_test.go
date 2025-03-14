package logger_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/leodido/go-syslog/v4/rfc3164"
	"github.com/mxmauro/logger"
	"github.com/mxmauro/logger/engines/syslog"
)

//------------------------------------------------------------------------------

func TestSysLogUDP(t *testing.T) {
	var serverErr error

	wg := sync.WaitGroup{}

	ctx, cancelCtx := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()

		serverErr = runMockSysLogUdpServer(ctx, t)
	}()

	lg := logger.Create(logger.Options{
		Level: logger.LogLevelDebug,
		DebugLevel: 1,
	})
	defer lg.Destroy()

	err := lg.AddSysLogEngine(syslog.Options{
		Host: "127.0.0.1",
		Port: 514,
	})
	if err != nil {
		t.Errorf("unable to initialize. [%v]", err)
		cancelCtx()
		wg.Wait()
		return
	}

	printTestMessages(lg)

	time.Sleep(3 * time.Second) // Let's give some time to process all
	cancelCtx()
	wg.Wait()

	if serverErr != nil {
		t.Errorf("server error. [%v]", serverErr)
	}
}

func TestSysLogTCP(t *testing.T) {
	var serverErr error

	wg := sync.WaitGroup{}

	ctx, cancelCtx := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()

		serverErr = runMockSysLogTcpServer(ctx, t)
	}()

	lg := logger.Create(logger.Options{
		Level: logger.LogLevelDebug,
		DebugLevel: 1,
	})
	defer lg.Destroy()

	err := lg.AddSysLogEngine(syslog.Options{
		Host:   "127.0.0.1",
		Port:   1468,
		UseTcp: true,
	})
	if err != nil {
		t.Errorf("unable to initialize. [%v]", err)
		cancelCtx()
		wg.Wait()
		return
	}

	printTestMessages(lg)

	time.Sleep(3 * time.Second) // Let's give some time to process all
	cancelCtx()
	wg.Wait()

	if serverErr != nil {
		t.Errorf("server error. [%v]", serverErr)
	}
}

//------------------------------------------------------------------------------
// Private methods

func runMockSysLogUdpServer(ctx context.Context, t *testing.T) error {
	var conn *net.UDPConn

	// Create UDP listener
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:514")
	if err != nil {
		return err
	}

	conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	// Set read buffer size
	err = conn.SetReadBuffer(1024)
	if err != nil {
		_ = conn.Close()
		return err
	}

	// Launch connection loop
	wg := sync.WaitGroup{}
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()

		buf := make([]byte, 1024)
		for {
			// Read message
			n, _, err2 := conn.ReadFrom(buf)
			if err2 == nil {
				if n == 0 {
					// Graceful shutdown
					return
				}
				// Ignore trailing control characters and NULs
				for ; n > 0 && buf[n-1] < 32; n-- {
				}
				if n > 0 {
					// Process message if any
					err2 = processMessage(t, buf[:n])
					if err2 != nil {
						errCh <- err2
						return
					}
				}
			} else {
				// On error, check if it is a network one
				var opError *net.OpError

				if errors.Is(err2, net.ErrClosed) {
					return
				}
				if errors.As(err2, &opError) && !opError.Temporary() && !opError.Timeout() {
					errCh <- err2
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Wait until shutdown if requested or some error happens
	select {
	case <-ctx.Done():
		err = nil
	case err = <-errCh:
	}

	// Shut down
	_ = conn.Close()
	wg.Wait()

	// Done
	return err
}

func runMockSysLogTcpServer(ctx context.Context, t *testing.T) error {
	var listener *net.TCPListener

	// Start TCP listener
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:1468")
	if err != nil {
		return err
	}

	listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	// Launch listener loop
	wg := sync.WaitGroup{}
	errCh := make(chan error)
	activeConnsMapLock := sync.RWMutex{}
	activeConnsMap := make(map[int]net.Conn)

	wg.Add(1)
	go func() {
		defer wg.Done()

		connIdx := 0

		for {
			conn, _ := listener.Accept()
			if conn != nil {
				connIdx += 1

				// Launch connection loop
				wg.Add(1)
				activeConnsMapLock.Lock()
				activeConnsMap[connIdx] = conn
				activeConnsMapLock.Unlock()

				go func() {
					defer func() {
						activeConnsMapLock.Lock()
						delete(activeConnsMap, connIdx)
						activeConnsMapLock.Unlock()
						wg.Done()
					}()

					onDivider := false
					buf := make([]byte, 1024)
					msg := make([]byte, 0)
					for {
						n, err2 := conn.Read(buf)
						if err2 == nil {
							if n == 0 {
								return
							}
							ofs := 0
							for ofs < n {
								if onDivider {
									for ; ofs < n && (buf[ofs] == '\r' || buf[ofs] == '\n'); ofs++ {
									}
									if ofs < n {
										onDivider = false
									}
								} else {
									startOfs := ofs
									for ; ofs < n && buf[ofs] != '\r' && buf[ofs] != '\n'; ofs++ {
									}
									if startOfs < ofs {
										msg = append(msg, buf[startOfs:ofs]...)
									}
									if ofs < n {
										onDivider = true
										if len(msg) > 0 {
											err2 = processMessage(t, msg)
											if err2 != nil {
												errCh <- err2
												return
											}
											msg = msg[:0]
										}
									}
								}
							}
						} else {
							// On error, check if it is a network one
							var opError *net.OpError

							if errors.Is(err2, net.ErrClosed) {
								return
							}
							if errors.As(err2, &opError) && !opError.Temporary() && !opError.Timeout() {
								errCh <- err2
								return
							}
							return
						}
					}
				}()
			} else {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}()

	// Wait until shutdown if requested or some error happens
	select {
	case <-ctx.Done():
		err = nil
	case err = <-errCh:
	}

	// Shut down
	_ = listener.Close()
	activeConnsMapLock.RLock()
	for _, v := range activeConnsMap {
		_ = v.Close()
	}
	activeConnsMapLock.RUnlock()

	wg.Wait()

	// Done
	return err
}

func processMessage(t *testing.T, msg []byte) error {
	// Parse the syslog message
	p := rfc3164.NewParser()
	_m, err := p.Parse(msg)
	if err != nil {
		return err
	}

	m := _m.(*rfc3164.SyslogMessage)
	if m.Message != nil {
		t.Logf("MockSysLogServer received message: %v", *m.Message)
	}

	return nil
}
