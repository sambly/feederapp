package main

import (
	"fmt"
	"log"
	"runtime/debug"
)

func (app *Application) logError(err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	log.Println(err.Error())
	app.errorLog.Output(2, trace)
}
