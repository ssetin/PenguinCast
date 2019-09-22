// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

type Logger interface {
	Error(format string, v ...interface{})
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warning(format string, v ...interface{})

	Access(format string, v ...interface{})
	Stat(format string, v ...interface{})
	Log(format string, v ...interface{})

	Close()
}
