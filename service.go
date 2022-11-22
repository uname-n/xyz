package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/robertkrimen/otto"
	"github.com/rs/zerolog/log"
)

var (
	upgrader = websocket.Upgrader{}

	errRuntimeForceStop = errors.New("RuntimeForceStop")
)

type Service struct {
	Outgoing chan []byte
	Incoming chan []byte
	Scripts  map[string]map[string]map[string]string
	Active   map[string]map[string]map[string]bool
	Timeout  int
}

func (S *Service) HandleFunc(w http.ResponseWriter, r *http.Request) {
	// upgrade connection to websocket
	conn, _ := upgrader.Upgrade(w, r, nil)

	// goroutine writing messages from Outgoing channel to websocket
	go func() {
		for {
			msg := <-S.Outgoing
			if err := conn.WriteMessage(1, msg); err != nil {
				return
			}
		}
	}()

	// goroutine reading messages from the websocket sending the to the Incoming channel
	go func() {
		for {
			// read incoming websocket messages
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Error().Err(err).Msg("unable to read incoming websocket message.")
			}

			// send message to Incoming channel
			S.Incoming <- msg
		}
	}()

	// run forever
	for {

		// get message from incoming channel
		msg := <-S.Incoming

		// parse incoming json from websocket message to an Event object
		e := Event{}
		if err := json.Unmarshal(msg, &e); err != nil {
			log.Error().Err(err).Msg("unable to parse incoming websocket message.")
		}

		// call run script on incoming Event
		S.RunScripts(e)
	}
}

func (S *Service) LoadScripts(dir string) {
	// find .js files with the directory structure /*/*/* within the specified parent dir
	files, err := filepath.Glob(filepath.Join(dir, "/*/*/*.js"))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to walk directory.")
	}

	for f := range files {

		// regex the found path for the required variables, channel, topic, and file
		path := strings.Replace(files[f], dir, "", 1)
		re := regexp.MustCompile(`/(?P<channel>.*)/(?P<topic>.*)/(?P<file>.*).js`)
		re_path := re.FindStringSubmatch(path)

		log.Info().Str("path", path).Msg("found")

		// read file data
		dat, err := os.ReadFile(files[f])
		if err != nil {
			log.Fatal().Str("path", path).Err(err).Msg("unable to read file.")
		}

		// create channel in Scripts and Active if it does not exist
		if _, channel_exists := S.Scripts[re_path[1]]; !channel_exists {
			S.Scripts[re_path[1]] = make(map[string]map[string]string)
			S.Active[re_path[1]] = make(map[string]map[string]bool)
		}

		// create topic in Scripts and Active if it does not exist
		if _, topic_exists := S.Scripts[re_path[1]][re_path[2]]; !topic_exists {
			S.Scripts[re_path[1]][re_path[2]] = make(map[string]string)
			S.Active[re_path[1]][re_path[2]] = make(map[string]bool)
		}

		// add file string data to scripts and set to inactive
		S.Scripts[re_path[1]][re_path[2]][re_path[3]] = string(dat)
		S.Active[re_path[1]][re_path[2]][re_path[3]] = false
	}
}

func (S *Service) RunScripts(e Event) {
	// for each script that matches the channel and topic
	for script := range S.Scripts[e.Channel][e.Topic] {

		// skip if script is already running
		if !S.Active[e.Channel][e.Topic][script] {

			// set script to active
			S.Active[e.Channel][e.Topic][script] = true

			// trigger in goroutine
			go func(s string) {
				path := e.Channel + "/" + e.Topic + "/" + s

				log.Info().Str("script", path).Msg("running.")

				// defer catch errors til end of function
				defer func() {
					if caught := recover(); caught != nil {
						if caught == errRuntimeForceStop {
							log.Error().Str("script", path).Msg("execution time exceeded, runtime force stopped.")
						} else {
							log.Error().Str("script", path).Err(caught.(error)).Msg("")
						}
					}

					// reset the active flag
					S.Active[e.Channel][e.Topic][s] = false
					log.Info().Str("script", path).Msg("finished.")
				}()

				// create new js runtime and interrupt channel
				vm := otto.New()
				vm.Interrupt = make(chan func(), 1)

				// override console.log to use zerolog
				vm.Set("console", map[string]interface{}{
					"log": func(call otto.FunctionCall) otto.Value {
						dat, err := call.Argument(0).MarshalJSON()
						if err != nil {
							log.Error().Str("script", path).Str("function", "console.log").Msg("unable to marshal json.")
						}
						log.Info().Str("script", path).Msg(string(dat))
						return otto.UndefinedValue()
					},
				})

				// create function, wait, to pause execution of js script
				vm.Set("wait", func(call otto.FunctionCall) otto.Value {
					timeout, err := call.Argument(0).ToInteger()
					if err != nil {
						log.Error().Str("script", path).Str("function", "wait").Msg("unable to parse argument to integer.")
					}
					time.Sleep(time.Duration(timeout) * time.Millisecond)
					return otto.UndefinedValue()
				})

				// create function, send.ws / send.internal , to send json data to the Outgoing or Incoming channel
				vm.Set("send", map[string]interface{}{
					"ws": func(call otto.FunctionCall) otto.Value {
						// parse json data from function call
						req, err := call.Argument(0).Object().MarshalJSON()
						if err != nil {
							log.Error().Str("script", path).Err(err).Str("function", "send.ws").Msg("unable to marshal json.")
						}

						// send json data to Events channel
						S.Outgoing <- req
						log.Info().Str("script", path).Str("function", "send.ws").Msg(string(req))

						return otto.UndefinedValue()
					},
					"internal": func(call otto.FunctionCall) otto.Value {
						// parse json data from function call
						req, err := call.Argument(0).Object().MarshalJSON()
						if err != nil {
							log.Error().Str("script", path).Err(err).Str("function", "send.internal").Msg("unable to marshal json.")
						}

						// send json data to Events channel
						S.Incoming <- req
						log.Info().Str("script", path).Str("function", "send.internal").Msg(string(req))

						return otto.UndefinedValue()
					},
				})

				// set the Event object as variable "e" for use in js scripts
				vm.Set("e", e)

				// create goroutine to interrupt runtime if it exceeds timeout
				go func() {
					time.Sleep(time.Duration(S.Timeout) * time.Second)
					vm.Interrupt <- func() {
						panic(errRuntimeForceStop)
					}
				}()

				// run the script within the js runtime
				vm.Run(S.Scripts[e.Channel][e.Topic][s])
			}(script)
		}
	}
}
