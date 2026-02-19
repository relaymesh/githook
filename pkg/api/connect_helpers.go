package api

import "log"

func logError(logger *log.Logger, message string, err error) {
	if logger == nil {
		return
	}
	logger.Printf("%s: %v", message, err)
}
