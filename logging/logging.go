package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

type Loggerr struct {
	InfoLog  *log.Logger
	ErrorLog *log.Logger
}

var MyLogger = &Loggerr{}

func InitLogger() {

	// infoLogFile, err := os.OpenFile("/data/log/info.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// //defer infoLogFile.Close()

	// errorLogFile, err := os.OpenFile("/data/log/error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// //defer errorLogFile.Close()

	logDir := "./log"
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	infoLogFile, err := os.OpenFile(filepath.Join(logDir, "info.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	// defer infoLogFile.Close()

	errorLogFile, err := os.OpenFile(filepath.Join(logDir, "error.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}

	MyLogger.InfoLog = log.New(infoLogFile, "INFO\t", log.Ldate|log.Ltime)
	MyLogger.ErrorLog = log.New(errorLogFile, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

}

func (l *Loggerr) ErrorOut(err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	// log.Println(err.Error())
	l.ErrorLog.Output(2, trace)
}
