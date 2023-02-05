package main

import (
	"encoding/json"
	"log"
	"niar/config"
	"niar/niar"
	"os"
	"os/signal"
	"syscall"
)

func waitStop() {
	// hook exit signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	s := <-sigs

	for _, n := range niars {
		n.Stop()
	}
	log.Printf("stop niar by signal [%v]", s)

	os.Exit(0)
}

var niars []*niar.Niar

func main() {
	file, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}

	c := config.Config{}

	err = json.Unmarshal(file, &c)
	if err != nil {
		panic(err)
	}

	for _, s := range c.Interfaces {
		n := niar.NewNiac(s)

		niars = append(niars, n)

		go n.Watch()
	}

	log.Printf("niar started...\n")

	waitStop()
}
