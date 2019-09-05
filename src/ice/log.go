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
