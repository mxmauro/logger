# logger

Yet another simple Go logger library.

* This is a fork of the original [RandLabs.IO's logger library](https://github.com/randlabs/go-logger).
  It contain some modified functionality and improvements.

## How to use

1. Import the library

```golang
import (
	"github.com/mxmauro/logger"
)
```

2. Then use `logger.Create` to create a logger object with desired options.
3. Add the desired engines (Console, File & SysLog) to the logger.
4. Optionally, you can also use the default logger which outputs only to console by accessing `logger.Default()`.

## Logger options:

The `Options` struct accepts several modifiers that affects the logger behavior:

| Field                        | Meaning                                                              |
|------------------------------|----------------------------------------------------------------------|
| `Level`                      | Set the initial logging level to use.                                |
| `DebugLevel`                 | Set the initial logging level for debug output to use.               |
| `UseLocalTime`               | Use the local computer time instead of UTC.                          |
| `SendSuccessAtErrorLogLevel` | Establishes when `Sucess(...)` sends the log as error or info level. |

#### Console engine Options:

| Field          | Meaning                                             |
|----------------|-----------------------------------------------------|
| `DisableColor` | Disable colored output if the terminal supports it. |

#### File engine Options:

| Field              | Meaning                                                                     |
|--------------------|-----------------------------------------------------------------------------|
| `Prefix`           | Filename prefix to use when a file is created. Defaults to the binary name. |
| `Directory`        | Destination directory to store log files.                                   |
| `DaysToKeep`       | Amount of days to keep old logs.                                            |
| `MaxFileSize`      | Set the maximum file size. Minimum is 10Kb. Unlimited if zero.              |
| `MaxFileVaultSize` | Set the maximum file storage size. Minimum is 1Mb. Unlimited if zero.       |

#### SysLog engine Options:

| Field                 | Meaning                                                                                   |
|-----------------------|-------------------------------------------------------------------------------------------|
| `AppName`             | Application name to use. Defaults to the binary name.                                     |
| `Host`                | Syslog server host name.                                                                  |
| `Port`                | Syslog server port. Defaults to 514, 1468 or 6514 depending on the network protocol used. |
| `UseTcp`              | Use TCP instead of UDP.                                                                   |
| `UseTls`              | Uses a secure connection. Implies TCP.                                                    |
| `UseRFC5424`          | Send messages in the new RFC 5424 format instead of the original RFC 3164 specification.  |
| `MaxMessageQueueSize` | Set the maximum amount of messages to keep in memory if connection to the server is lost. |
| `TlsConfig`           | An optional pointer to a `tls.Config` object to provide the TLS configuration for use.    |

## Example

```golang
package example

import (
	"fmt"

	"github.com/mxmauro/logger"
	"github.com/mxmauro/logger/engines/file"
)

// Define a custom JSON message. Timestamp and level will be automatically added by the logger.
type JsonMessage struct {
	Message string `json:"message"`
}

func main() {
	// Create the logger
	lg := logger.Create(logger.Options{
		Level:      logger.LogLevelDebug,
		DebugLevel: 1,
	})
	defer lg.Destroy()

	err := lg.AddFileEngine(file.Options{
		Directory:  "./logs",
		DaysToKeep: 7,
	})
	if err != nil {
		// Use default logger to send the error
		logger.Default().Error(fmt.Sprintf("unable to initialize. [%v]", err))
		return
	}

	// Send some logs using the plain text format 
	lg.Error("This is an error message sample")
	lg.Warning("This is a warning message sample")
	lg.Info("This is an information message sample")
	lg.Debug(1, "This is a debug message sample at level 1 which should be printed")
	lg.Debug(2, "This is a debug message sample at level 2 which should NOT be printed")

	// Send some other logs using the JSON format 
	lg.Error(JsonMessage{
		Message: "This is an error message sample",
	})
	lg.Warning(JsonMessage{
		Message: "This is a warning message sample",
	})
	lg.Info(JsonMessage{
		Message: "This is an information message sample",
	})
	lg.Debug(1, JsonMessage{
		Message: "This is a debug message sample at level 1 which should be printed",
	})
	lg.Debug(2, JsonMessage{
		Message: "This is a debug message sample at level 2 which should NOT be printed",
	})
}
```

## License

Apache 2.0. See [LICENSE](/LICENSE) file for details.
