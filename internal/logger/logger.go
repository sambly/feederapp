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

func InitLogger(debug, production bool) {

	log.SetOutput(ioutil.Discard) // Send all logs to nowhere by default
	log.SetReportCaller(true)

	//Общая настройка CallerPrettyfier
	callerPrettyfier := func(f *runtime.Frame) (string, string) {
		// Получаем только имя файла без полного пути
		fileName := filepath.Base(f.File)
		return "", fileName + ":" + strconv.Itoa(f.Line)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	projectRoot := filepath.Join(wd, "../..")
	logDir := filepath.Join(projectRoot, "log")

	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	logFile, err := os.OpenFile(filepath.Join(logDir, "app.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
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
		LogLevels: []logrus.Level{logrus.ErrorLevel},
	}
	log.AddHook(consoleHook)

	// Устанавливаем основной уровень логирования для логгера
	if debug {
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}
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

	// if production {
	// 	log.SetFormatter(&logrus.JSONFormatter{})
	// } else {
	// 	// The TextFormatter is default, you don't actually have to do this.
	// 	log.SetFormatter(&logrus.TextFormatter{})
	// }

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

func PrintStackTrace() {
	// Максимальное количество уровней стека, которое мы хотим проверить
	const maxDepth = 10

	for i := 0; i < maxDepth; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Выводим информацию о файле и строке
		fmt.Printf("Level %d: %s:%d\n", i, filepath.Base(file), line)
	}
}
