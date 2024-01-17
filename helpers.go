package main

import (
	"log"
)

func (app *Application) logError(err error) {
	log.Println(err.Error())
}
