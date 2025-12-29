package worker

import "log"

type Logger interface {
	Printf(format string, args ...interface{})
}

type stdLogger struct{}

func (stdLogger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
