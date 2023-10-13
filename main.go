package main

import (
	"encoding/json"
	"log"
	"net"
	"netcatcher/config"
	"netcatcher/netcatcher"
	"os"
	"os/signal"
	"syscall"
)

func waitStop() {
	// hook exit signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	s := <-sigs

	for _, n := range netcatchers {
		n.Stop()
	}
	log.Printf("stop netcatcher by signal [%v]", s)

	os.Exit(0)
}

var netcatchers []*netcatcher.NetCatcher

func main() {

	interfaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, i := range interfaces {
		addrs, err := i.Addrs()
		if err != nil {
			panic(err)
		}
		log.Printf("%s: ", i.Name)
		for _, a := range addrs {
			log.Printf("\t%s", a.String())
		}
	}

	configPath := "config.json"

	// get -c argument
	if len(os.Args) > 2 {
		if os.Args[1] == "-c" {
			if len(os.Args) < 2 {
				panic("config file not found")
			}
			configPath = os.Args[2]
		}
	}

	log.Printf("config file: %s\n", configPath)

	file, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	c := config.Config{}

	err = json.Unmarshal(file, &c)
	if err != nil {
		panic(err)
	}

	for _, s := range c.Interfaces {
		n := netcatcher.NewNetCatcher(s)

		netcatchers = append(netcatchers, n)

		go n.Watch()
	}

	log.Printf("netcatcher started...\n")

	waitStop()
}
