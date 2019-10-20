package monerotipbot

import (
	"flag"
	"fmt"
	"log"
)

// ParseArguments parses all CLI arguments
func ParseArguments() (*Config, error) {
	// Did the user ask for help?
	parseErr := fmt.Errorf("")

	var (
		configpath string
		debug      bool
		nolog      bool
	)
	flag.StringVar(&configpath, "c", "", "")
	flag.BoolVar(&debug, "debug", false, "")
	flag.Parse()
	if !flag.Parsed() {
		return nil, parseErr
	}

	// Validate arguments and save configuration
	config, err := saveConfig(configpath, debug, nolog)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return config, nil
}

func saveConfig(configpath string, debug, nolog bool) (*Config, error) {
	conf := NewConfig()

	conf.SetConfig(configpath)
	conf.SetDebug(debug)

	return conf, nil
}
