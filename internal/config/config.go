package config

import "flag"

type Config struct {
	ServerAddr string
	BaseURL    string
}

func Load() *Config {
	config := &Config{}
	flag.StringVar(&config.ServerAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&config.BaseURL, "b", "http://localhost:8080/", "baseURL of shortened url")
	flag.Parse()
	return config
}
