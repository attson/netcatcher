package main

import (
	"encoding/json"
	"flag"
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
	configPath := flag.String("c", "config.json", "config file path")
	logPath := flag.String("l", "", "log file path")
	flag.Parse()

	if *logPath != "" {
		open, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		log.SetOutput(open)
	}

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

	log.Printf("config file: %s\n", *configPath)

	file, err := os.ReadFile(*configPath)
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
