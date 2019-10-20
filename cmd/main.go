package main

import (
	_ "image/jpeg"
	"log"
	"os"

	"github.com/omani/monerotipbot"
)

func main() {
	// defer catchpanics()
	log.SetOutput(os.Stdout)

	// parse cli arguments
	conf, err := monerotipbot.ParseArguments()
	if err != nil {
		os.Exit(1)
	}

	// initiate a new monerotipbot instance
	bot, err := monerotipbot.NewBot(conf)
	if err != nil {
		log.Fatal(err)
	}

	// start monerotipbot
	bot.Run()
}

func catchpanics() {
	if err := recover(); err != nil {
		log.Printf("Fatal error (panic): %s", err)
	}
}
