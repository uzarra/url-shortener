package config

type Config struct {
	ServerAddr string
	BaseURL    string
}

func Load() *Config {
	return &Config{
		ServerAddr: ":8080",
		BaseURL:    "http://localhost:8080/",
	}
}
