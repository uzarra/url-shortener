package config

import (
	"testing"
	"time"
)

func TestLoad_DeleterSettings(t *testing.T) {
	tests := []struct {
		name           string
		env            map[string]string
		args           []string
		wantBatchSize  int
		wantInterval   time.Duration
		wantMaxWorkers int
	}{
		{
			name:           "defaults",
			wantBatchSize:  defaultDeleteBatchSize,
			wantInterval:   defaultDeleteFlushInterval,
			wantMaxWorkers: defaultDeleteMaxWorkers,
		},
		{
			name: "from flags",
			args: []string{
				"-delete-batch-size", "50",
				"-delete-flush-interval", "500ms",
				"-delete-max-workers", "4",
			},
			wantBatchSize:  50,
			wantInterval:   500 * time.Millisecond,
			wantMaxWorkers: 4,
		},
		{
			name: "from env",
			env: map[string]string{
				"DELETE_BATCH_SIZE":     "20",
				"DELETE_FLUSH_INTERVAL": "2s",
				"DELETE_MAX_WORKERS":    "8",
			},
			wantBatchSize:  20,
			wantInterval:   2 * time.Second,
			wantMaxWorkers: 8,
		},
		{
			name: "env wins over flags",
			env: map[string]string{
				"DELETE_BATCH_SIZE":     "20",
				"DELETE_FLUSH_INTERVAL": "2s",
				"DELETE_MAX_WORKERS":    "8",
			},
			args: []string{
				"-delete-batch-size", "50",
				"-delete-flush-interval", "500ms",
				"-delete-max-workers", "4",
			},
			wantBatchSize:  20,
			wantInterval:   2 * time.Second,
			wantMaxWorkers: 8,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t, envKeys...)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			resetCLI(t, tt.args...)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() returned error: %v", err)
			}
			if cfg.DeleteBatchSize != tt.wantBatchSize {
				t.Errorf("DeleteBatchSize = %d, want %d", cfg.DeleteBatchSize, tt.wantBatchSize)
			}
			if cfg.DeleteFlushInterval != tt.wantInterval {
				t.Errorf("DeleteFlushInterval = %s, want %s", cfg.DeleteFlushInterval, tt.wantInterval)
			}
			if cfg.DeleteMaxWorkers != tt.wantMaxWorkers {
				t.Errorf("DeleteMaxWorkers = %d, want %d", cfg.DeleteMaxWorkers, tt.wantMaxWorkers)
			}
		})
	}
}

func TestLoad_InvalidDeleterSettings(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		args []string
	}{
		{name: "zero batch size flag", args: []string{"-delete-batch-size", "0"}},
		{name: "negative batch size env", env: map[string]string{"DELETE_BATCH_SIZE": "-1"}},
		{name: "unparsable batch size env", env: map[string]string{"DELETE_BATCH_SIZE": "many"}},
		{name: "zero interval flag", args: []string{"-delete-flush-interval", "0s"}},
		{name: "negative interval env", env: map[string]string{"DELETE_FLUSH_INTERVAL": "-1s"}},
		{name: "unparsable interval env", env: map[string]string{"DELETE_FLUSH_INTERVAL": "soon"}},
		{name: "zero max workers flag", args: []string{"-delete-max-workers", "0"}},
		{name: "negative max workers env", env: map[string]string{"DELETE_MAX_WORKERS": "-3"}},
		{name: "unparsable max workers env", env: map[string]string{"DELETE_MAX_WORKERS": "lots"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t, envKeys...)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			resetCLI(t, tt.args...)

			if cfg, err := Load(); err == nil {
				t.Fatalf("Load() error = nil, want error (cfg = %+v)", cfg)
			}
		})
	}
}
