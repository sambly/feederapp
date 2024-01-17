package log

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
)

type Loggerr struct {
	InfoLog  *log.Logger
	ErrorLog *log.Logger
}

var MyLogger = &Loggerr{}

func InitLogger() {

	infoLogFile, err := os.OpenFile("log/data/info.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	//defer infoLogFile.Close()

	errorLogFile, err := os.OpenFile("log/data/error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	//defer errorLogFile.Close()

	MyLogger.InfoLog = log.New(infoLogFile, "INFO\t", log.Ldate|log.Ltime)
	MyLogger.ErrorLog = log.New(errorLogFile, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

}

func (l *Loggerr) ErrorOut(err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	log.Println(err.Error())
	l.ErrorLog.Output(2, trace)
}
