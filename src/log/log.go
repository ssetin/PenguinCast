// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package log

import (
	"fmt"
	"log"
	"os"
)

type LogsLevel int

const (
	LevelError   = 1
	LevelWarning = 2
	LevelInfo    = 3
	LevelDebug   = 4
)

type IceLogger struct {
	level     LogsLevel
	logError  *log.Logger
	logAccess *log.Logger

	logErrorFile  *os.File
	logAccessFile *os.File
	statFile      *os.File
}

func NewLogger(level LogsLevel, logsPath string) (*IceLogger, error) {
	newLogger := &IceLogger{
		level: level,
	}

	errorFileName := logsPath + "/error.log"
	accessFileName := logsPath + "/access.log"
	statFileName := logsPath + "/stat.log"

	var err error
	newLogger.logErrorFile, err = os.OpenFile(errorFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	newLogger.logAccessFile, err = os.OpenFile(accessFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	if len(statFileName) > 0 {
		newLogger.statFile, err = os.OpenFile(statFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprintln(newLogger.statFile, "#Time	#Listeners	#CpuUsage	#MemUsage")
	}

	newLogger.logError = log.New(newLogger.logErrorFile, "", log.Ldate|log.Ltime)
	newLogger.logAccess = log.New(newLogger.logAccessFile, "", 0)

	return newLogger, nil
}

func (l *IceLogger) Error(errorLevel LogsLevel, format string, v ...interface{}) {
	if errorLevel <= l.level {
		var mark string
		switch errorLevel {
		case 1:
			mark = "Error: "
		case 2:
			mark = "Warn: "
		case 3:
			mark = "Info: "
		case 4:
			mark = "Debug: "
		}
		l.logError.Print(mark)
		l.logError.Printf(format, v...)
	}
}

func (l *IceLogger) Access(format string, v ...interface{}) {
	l.logAccess.Printf(format, v...)
}

func (l *IceLogger) Log(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *IceLogger) Stat(format string, v ...interface{}) {
	_, _ = fmt.Fprintf(l.statFile, format, v)
}

func (l *IceLogger) Close() {
	_ = l.logErrorFile.Close()
	_ = l.logAccessFile.Close()
	if l.statFile != nil {
		_ = l.statFile.Close()
	}
}
