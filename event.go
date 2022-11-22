package main

type Event struct {
	Channel string
	Topic   string
	Message map[string]interface{}
}
