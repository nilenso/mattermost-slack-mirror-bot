package main

import (
	"fmt"
	"io"
	"log"
	"time"
)

type LogLevel int

const (
	Error LogLevel = iota
	Info
	Debug
)

var LogLevels = map[LogLevel]string{
	Error: "ERR",
	Info:  "INF",
	Debug: "DBG",
}

type Logger struct {
	del      *log.Logger
	location *time.Location
}

func NewLogger(location *time.Location, writer io.Writer) *Logger {
	return &Logger{
		location: location,
		del:      log.New(writer, "", 0),
	}
}

func (logger *Logger) log(level LogLevel, format string, args ...interface{}) {
	now := time.Now().In(logger.location).Format("2006-01-02 15:04:05")
	format = fmt.Sprintf("[%s][%s] %s\n", LogLevels[level], now, format)
	logger.del.Printf(format, args...)
}

func (logger *Logger) error(format string, args ...interface{}) {
	logger.log(Error, format, args...)
}

func (logger *Logger) info(format string, args ...interface{}) {
	logger.log(Info, format, args...)
}

func (logger *Logger) debug(format string, args ...interface{}) {
	logger.log(Debug, format, args...)
}
