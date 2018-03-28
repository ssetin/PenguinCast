package icyserver

import (
	"log"
	"os"
)

/*
	Log Level
	1 - Errors; 2 - Debug
*/

func (i *IcyServer) initLog() error {
	var err error
	i.logErrorFile, err = os.OpenFile(i.Props.Paths.Log+"error.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	i.logAccessFile, err = os.OpenFile(i.Props.Paths.Log+"access.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	i.logError = log.New(i.logErrorFile, "", log.Ldate|log.Ltime)
	i.logAccess = log.New(i.logAccessFile, "", 0)

	return nil
}

func (i *IcyServer) printError(errorlevel int, format string, v ...interface{}) {
	if errorlevel <= i.Props.Logging.Loglevel {
		var mark string
		if errorlevel == 1 {
			mark = "Error: "
		} else {
			mark = "Debug: "
		}
		i.logError.Printf(mark+format, v...)
	}
}

func (i *IcyServer) printAccess(format string, v ...interface{}) {
	i.logAccess.Printf(format, v...)
}
