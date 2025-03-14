package logger

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

//------------------------------------------------------------------------------

type logType uint

const (
	logTypeSuccess logType = iota
	logTypeError
	logTypeWarning
	logTypeInfo
	logTypeDebug
)

//------------------------------------------------------------------------------

func (lg *Logger) log(obj interface{}, jsonLevel string, _type logType) {
	msg, isJSON, ok := parseObj(obj)
	if !ok {
		return
	}

	now := lg.getTimestamp()
	raw := false
	if isJSON {
		msg = addPayloadToJSON(msg, now, jsonLevel)
		raw = true
	}

	switch _type {
	case logTypeSuccess:
		for _, engine := range lg.engines {
			engine.Success(now, msg, raw, lg.sendSuccessAtErrorLogLevel)
		}
	case logTypeError:
		for _, engine := range lg.engines {
			engine.Error(now, msg, raw)
		}
	case logTypeWarning:
		for _, engine := range lg.engines {
			engine.Warning(now, msg, raw)
		}
	case logTypeInfo:
		for _, engine := range lg.engines {
			engine.Info(now, msg, raw)
		}
	case logTypeDebug:
		for _, engine := range lg.engines {
			engine.Debug(now, msg, raw)
		}
	}
}

func (lg *Logger) getTimestamp() time.Time {
	now := time.Now()
	if !lg.useLocalTime {
		now = now.UTC()
	}
	return now
}

//------------------------------------------------------------------------------

func parseObj(obj interface{}) (msg string, isJSON bool, ok bool) {
	// Quick check for strings, structs or pointer to strings or structs
	refObj := reflect.ValueOf(obj)
	switch refObj.Kind() {
	case reflect.Ptr:
		if !refObj.IsNil() {
			switch refObj.Elem().Kind() {
			case reflect.String:
				msg = *(obj.(*string))
				ok = true

			case reflect.Struct:
				// Marshal struct
				b, err := json.Marshal(obj)
				if err == nil {
					msg = string(b)
					isJSON = true
					ok = true
				}
			}
		}

	case reflect.String:
		msg = obj.(string)
		ok = true

	case reflect.Struct:
		// Marshal struct
		b, err := json.Marshal(obj)
		if err == nil {
			msg = string(b)
			isJSON = true
			ok = true
		}
	}

	// Done
	return
}

func addPayloadToJSON(s string, now time.Time, level string) string {
	if len(s) < 2 || s[0] != '{' {
		return s // Cannot modify if not an encoded object
	}

	sb := strings.Builder{}
	_, _ = sb.WriteString(s[:1])
	_, _ = sb.WriteString(fmt.Sprintf(`"timestamp":"%v","level":"%v"`, now.Format("2006-01-02 15:04:05.000"), level))
	if s[1] != '}' {
		_, _ = sb.WriteString(",") // Add the comma separator if not an empty json object
	}
	_, _ = sb.WriteString(s[1:])

	// Return modified string
	return sb.String()
}
