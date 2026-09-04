package config

import "flag"

type Config struct {
	ServerAddr string
	BaseURL    string
}

func Load() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.ServerAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080/", "baseURL of shortened url")
	flag.Parse()
	return cfg
}
