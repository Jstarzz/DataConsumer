package configs

import (
	"encoding/json"
	"os"
	"runtime"
)

// Config holds application configuration
type Config struct {
	// DataSources is a list of URLs to download data from
	DataSources []string `json:"data_sources"`

	// TargetRate is the target data consumption rate in MB/minute
	TargetRate int `json:"target_rate"`

	// Duration is how long to run in minutes (0 for indefinite)
	Duration int `json:"duration"`

	// VerboseLogging enables detailed logging
	VerboseLogging bool `json:"verbose_logging"`

	// SaveMetrics enables saving metrics to a file
	SaveMetrics bool `json:"save_metrics"`

	// MetricsFile is the file to save metrics to
	MetricsFile string `json:"metrics_file"`

	// ConcurrencyFactor adjusts the number of workers relative to CPU cores
	ConcurrencyFactor int `json:"concurrency_factor"`

	// UseRandomization adds random parameters to requests to avoid caching
	UseRandomization bool `json:"use_randomization"`

	// RequestTimeout is the timeout for HTTP requests in seconds
	RequestTimeout int `json:"request_timeout"`
}

// DefaultConfig returns the default configuration optimized for 200MB/min
func DefaultConfig() *Config {
	return &Config{
		// Use multiple data sources for reliability
		DataSources: []string{
			"https://speed.cloudflare.com/10mb.bin",
			"https://speed.cloudflare.com/100mb.bin",
			"https://cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js",
			"https://ajax.googleapis.com/ajax/libs/jquery/3.6.0/jquery.min.js",
			"https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.0/jquery.min.js",
		},
		TargetRate:        200, // 200 MB per minute
		Duration:          0,   // Run indefinitely by default
		VerboseLogging:    false,
		SaveMetrics:       true,
		MetricsFile:       "dataconsumer_metrics.json",
		ConcurrencyFactor: runtime.NumCPU(),
		UseRandomization:  true,
		RequestTimeout:    60, // 60 seconds
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := DefaultConfig()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves the current configuration to a file
func SaveConfig(config *Config, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}
