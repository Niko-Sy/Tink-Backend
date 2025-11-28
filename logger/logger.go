package logger

import "time"

type LogMessage struct {
	err       *error
	message   string
	timestamp time.Time
}

func Logger(e *error, message string) {

}
