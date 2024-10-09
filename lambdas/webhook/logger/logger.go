package logger

import (
	"log"
	"sync"
)

type Logger struct {
}

var instance *Logger

var once sync.Once

func GetLogger() *Logger {
	instance.initialize()
	return instance
}

/*
Using 'once' is likely overkill but using this for thread safety
to ensure that this initialization process is only done
once
*/
func (l *Logger) initialize() {
	once.Do(func() {
		/**
		  This sets the following flags on a the logger
		  LUTC -> sets date and time to be in UTC
		  Ldate -> enables data
		  Ltime -> enables time
		  Lshortfile -> enables short filename and line number

		  Resulting log message looks something like the following:
		  2024/10/05 01:02:03 <PREFIX> <log_message>
		  **/
		log.SetFlags(log.LUTC | log.Ldate | log.Ltime | log.Lmsgprefix)
		log.SetPrefix("")
	})
}

// logs a message with a specified <prefix>
func (l *Logger) PREFIX(prefix string, format string, args ...interface{}) {
	l.initialize()
	log.SetPrefix(prefix)
	log.Printf(format, args...)
	log.SetPrefix("")
}

// logs a message with an INFO prefix
func (l *Logger) INFO(format string, args ...interface{}) {
	l.initialize()
	log.SetPrefix("INFO: ")
	log.Printf(format, args...)
	log.SetPrefix("")
}

// logs a message with an WARN prefix
func (l *Logger) WARN(format string, args ...interface{}) {
	l.initialize()
	log.SetPrefix("WARN: ")
	log.Printf(format, args...)
	log.SetPrefix("")
}

// logs a message with an ERROR prefix and return the error
func (l *Logger) ERROR(format string, args ...interface{}) {
	l.initialize()
	log.SetPrefix("ERROR: ")
	log.Printf(format, args...)
	log.SetPrefix("")
}
