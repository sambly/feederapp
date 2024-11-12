package logger

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/sirupsen/logrus"
)

type LogHook struct {
	Writer    io.Writer
	LogLevels []logrus.Level
}

func (hook *LogHook) Fire(entry *logrus.Entry) error {

	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = hook.Writer.Write([]byte(line))
	return err
}

func (hook *LogHook) Levels() []logrus.Level {
	return hook.LogLevels
}

var log = logrus.New()

// Общая настройка CallerPrettyfier
var callerPrettyfier = func(f *runtime.Frame) (string, string) {
	// Получаем только имя файла без полного пути
	fileName := filepath.Base(f.File)
	return "", fileName + ":" + strconv.Itoa(f.Line)
}

func InitLogger(debug, production bool) error {

	log.SetOutput(ioutil.Discard) // Send all logs to nowhere by default
	log.SetReportCaller(true)

	if err := os.MkdirAll("log", os.ModePerm); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}
	logFile, err := os.OpenFile("./log/app.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed OpenFile log: %v", err)
	}

	// Настройка хука для логирования в файл
	fileHook := &LogHook{
		Writer:    logFile,
		LogLevels: logrus.AllLevels,
	}
	log.AddHook(fileHook)

	// Настройка хука для логирования в консоль
	consoleHook := &LogHook{
		Writer:    os.Stdout,
		LogLevels: []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel},
	}
	log.AddHook(consoleHook)

	LoggerSetLevel(debug)
	LoggerSetFormatter(production)

	return nil
}

func LoggerSetLevel(debug bool) {
	// Устанавливаем основной уровень логирования для логгера
	if debug {
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}
}

func LoggerSetFormatter(production bool) {
	if production {
		//Настройка JSON форматтера для production
		log.SetFormatter(&logrus.JSONFormatter{
			CallerPrettyfier: callerPrettyfier,
		})
	} else {
		// Настройка TextFormatter для development
		log.SetFormatter(&logrus.TextFormatter{
			CallerPrettyfier: callerPrettyfier,
		})
	}
}

func GetLogger() *logrus.Logger {
	return log
}

func AddFields(fields map[string]interface{}) *logrus.Entry {
	return log.WithFields(logrus.Fields(fields))
}

func AddFieldsEmpty() *logrus.Entry {
	return log.WithFields(logrus.Fields{})
}
