package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	debug   int
	verbose bool
	output  string

	addr    string
	path    string
	timeout int
)

func init() {
	// get flag data
	flag.IntVar(&debug, "debug", 2, "set log level. default error. (default: 2)")
	flag.BoolVar(&verbose, "verbose", false, "set output to stdout.")
	flag.StringVar(&output, "output", "activity.log", "path to output log.")

	flag.StringVar(&addr, "addr", ":8080", "websocket address. default: :8080")
	flag.StringVar(&path, "path", "./scripts", "path to directory containing js scripts")
	flag.IntVar(&timeout, "execution_time", 15, "set execution time limit.")

	flag.Parse()

	// configure zerolog based on flag data
	if !verbose {
		file, err := os.Create(output)
		if err != nil {
			panic(err)
		}
		log.Logger = zerolog.New(file).With().Logger()
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	zerolog.SetGlobalLevel(zerolog.Level(debug))
}

func main() {
	// create service
	s := Service{
		Outgoing: make(chan []byte),
		Incoming: make(chan []byte),
		Scripts:  make(map[string]map[string]map[string]string),
		Active:   make(map[string]map[string]map[string]bool),
		Timeout:  timeout,
	}

	// load scripts from path specified in flags
	s.LoadScripts(path)

	// set the http handle function
	http.HandleFunc("/", s.HandleFunc)

	// start the server
	log.Fatal().Err(http.ListenAndServe(addr, nil)).Msg("")
}
