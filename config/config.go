package config

type Interface struct {
	Name   string   `json:"name"`
	Routes []string `json:"routes"`
}

type Config struct {
	Interfaces []Interface `json:"interfaces"`
}
