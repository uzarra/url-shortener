package config

import (
	"flag"
	"io"
	"os"
	"testing"
)

const (
	defaultAddr = ":8080"
	defaultBase = "http://localhost:8080/"
)

func resetCLI(t *testing.T, args ...string) {
	t.Helper()

	origArgs := os.Args
	origCmdLine := flag.CommandLine

	flag.CommandLine = flag.NewFlagSet(origArgs[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = append([]string{origArgs[0]}, args...)

	t.Cleanup(func() {
		os.Args = origArgs
		flag.CommandLine = origCmdLine
	})
}

func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if old, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { _ = os.Setenv(key, old) })
		}
		_ = os.Unsetenv(key)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		args     []string
		wantAddr string
		wantBase string
	}{
		{
			name:     "defaults when neither env nor flags are set",
			wantAddr: defaultAddr,
			wantBase: defaultBase,
		},
		{
			name: "env vars are used",
			env: map[string]string{
				"SERVER_ADDRESS": ":9090",
				"BASE_URL":       "http://env.kz/",
			},
			wantAddr: ":9090",
			wantBase: "http://env.kz/",
		},
		{
			name:     "flags are used when env is absent",
			args:     []string{"-a", ":7070", "-b", "http://cli.kz/"},
			wantAddr: ":7070",
			wantBase: "http://cli.kz/",
		},
		{
			name:     "env for serverAddr, flag for baseUrl",
			env:      map[string]string{"SERVER_ADDRESS": ":9090"},
			args:     []string{"-b", "http://cli.kz/"},
			wantAddr: ":9090",
			wantBase: "http://cli.kz/",
		},
		{
			name:     "env for baseUrl, flag for serverAddr",
			env:      map[string]string{"BASE_URL": "http://env.kz/"},
			args:     []string{"-a", ":8090"},
			wantAddr: ":8090",
			wantBase: "http://env.kz/",
		},
		{
			name:     "default baseUrl",
			args:     []string{"-a", ":8090"},
			wantAddr: ":8090",
			wantBase: "http://localhost:8080/",
		},
		{
			name:     "default serverAddr",
			args:     []string{"-b", "http://cli.kz/"},
			wantAddr: ":8080",
			wantBase: "http://cli.kz/",
		},
		{
			name: "both env vars and flags exist but env vars win",
			env: map[string]string{
				"SERVER_ADDRESS": ":9090",
				"BASE_URL":       "http://env.kz/",
			},
			args: []string{
				"-a", ":7070",
				"-b", "http://cli.kz/",
			},
			wantAddr: ":9090",
			wantBase: "http://env.kz/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t, "SERVER_ADDRESS", "BASE_URL")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			resetCLI(t, tt.args...)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() returned error: %v", err)
			}
			if cfg.ServerAddr != tt.wantAddr {
				t.Errorf("ServerAddr = %q, want %q", cfg.ServerAddr, tt.wantAddr)
			}
			if cfg.BaseURL != tt.wantBase {
				t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, tt.wantBase)
			}
		})
	}
}
