package internal

import (
	"fmt"
	"log"
	"strings"
)

type Logger interface {
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Report(msg string, args ...interface{})
	Error(msg string, err error, args ...interface{})
}

type raftLogger struct {
	fieldsStr string
}

type logLevel string

const (
	infoLevel   logLevel = "INFO"
	debugLevel  logLevel = "DEBUG"
	reportLevel logLevel = "REPORT"
	errorLevel  logLevel = "ERROR"
)

func NewLogger(fields map[string]interface{}) *raftLogger {
	rl := raftLogger{}
	if fields != nil {
		for k, v := range fields {
			rl.fieldsStr = fmt.Sprintf("%v %v=%v", rl.fieldsStr, k, v)
		}

		rl.fieldsStr = strings.TrimSpace(rl.fieldsStr)
	}

	return &rl
}

func (rl *raftLogger) Info(msg string, args ...interface{}) {
	keys := ""
	for i := len(args) - 1; i >= 0; i -= 2 {
		key := args[i-1]
		val := args[i]

		keys = fmt.Sprintf("%v %v=%v", keys, key, val)
	}

	rl.print(infoLevel, fmt.Sprintf("%v %v msg=%v", rl.fieldsStr, keys, msg))
}

func (rl *raftLogger) Debug(msg string, args ...interface{}) {
	keys := ""
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]

		keys = fmt.Sprintf("%v %v=%v", keys, key, value)
	}

	rl.print(debugLevel, fmt.Sprintf("%v msg=%v%v", rl.fieldsStr, msg, keys))
}

func (rl *raftLogger) Report(msg string, args ...interface{}) {
	keys := ""
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]

		keys = fmt.Sprintf("%v %v=%v", keys, key, value)
	}

	rl.print(reportLevel, fmt.Sprintf("%v %v", rl.fieldsStr, keys))

}

func (rl *raftLogger) Error(msg string, err error, args ...interface{}) {
	keys := ""
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]

		keys = fmt.Sprintf("%v %v=%v", keys, key, value)
	}

	rl.print(errorLevel, fmt.Sprintf("%v error=%s msg=%v%v", rl.fieldsStr, err, msg, keys))
}

func (rl *raftLogger) print(level logLevel, msg string) {
	log.Printf("%v %v", level, strings.TrimSpace(msg))
}
