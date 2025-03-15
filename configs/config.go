package configs

import (
	"encoding/json"
	"os"
	"runtime"
)

type Config struct {
	DataSources       []string `json:"data_sources"`
	TargetRate        int      `json:"target_rate"`
	Duration          int      `json:"duration"`
	VerboseLogging    bool     `json:"verbose_logging"`
	SaveMetrics       bool     `json:"save_metrics"`
	MetricsFile       string   `json:"metrics_file"`
	ConcurrencyFactor int      `json:"concurrency_factor"`
	UseRandomization  bool     `json:"use_randomization"`
	RequestTimeout    int      `json:"request_timeout"`
}

func DefaultConfig() *Config {
	return &Config{
		DataSources: []string{
			"https://speed.cloudflare.com/1000mb.bin",                                                   // 1 GB
			"https://ftp.arnes.si/software/ubuntu-releases/20.04/ubuntu-20.04.3-desktop-amd64.iso",      // ~2.5 GB
			"https://releases.ubuntu.com/20.04.4/ubuntu-20.04.4-desktop-amd64.iso",                      // ~2.5 GB
			"https://ftp.gnu.org/gnu/gcc/gcc-11.1.0/gcc-11.1.0.tar.xz",                                  // ~100 MB
			"https://download.blender.org/release/Blender2.93/blender-2.93.0-linux64.tar.xz",            // ~200 MB
			"https://ftp.mozilla.org/pub/firefox/releases/90.0/linux-x86_64/en-US/firefox-90.0.tar.bz2", // ~70 MB
			"https://ftp.gnu.org/gnu/binutils/binutils-2.36.1.tar.xz",                                   // ~20 MB
		},
		TargetRate:        1024,
		Duration:          0,
		VerboseLogging:    false,
		SaveMetrics:       true,
		MetricsFile:       "dataconsumer_metrics.json",
		ConcurrencyFactor: runtime.NumCPU(),
		UseRandomization:  true,
		RequestTimeout:    60,
	}
}

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
