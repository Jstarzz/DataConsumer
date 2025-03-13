package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"dataconsumer/configs"
	"dataconsumer/internal/consumer"
	"dataconsumer/internal/metrics"
)

func main() {
	// Parse command line arguments
	configPath := flag.String("config", "", "Path to configuration file")
	targetRate := flag.Int("rate", 200, "Target data consumption rate in MB/minute")
	duration := flag.Int("duration", 0, "Duration to run in minutes (0 for indefinite)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	outputMetrics := flag.String("metrics", "dataconsumer_metrics.json", "Path to save metrics")
	saveInterval := flag.Int("save-interval", 60, "Save metrics every N seconds")
	flag.Parse()

	// Display banner
	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║            DATA CONSUMER v2.0              ║")
	fmt.Println("║  High-Performance Network Data Consumer    ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Printf("Running on %s with %d CPU cores\n\n", runtime.GOOS, runtime.NumCPU())

	// Initialize configuration
	config := configs.DefaultConfig()
	if *configPath != "" {
		var err error
		config, err = configs.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}
	}
	config.TargetRate = *targetRate
	config.Duration = *duration
	config.VerboseLogging = *verbose
	config.MetricsFile = *outputMetrics

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector()

	// Enable metrics logging
	if config.SaveMetrics {
		logFile := fmt.Sprintf("dataconsumer_log_%s.csv",
			time.Now().Format("20060102_150405"))
		if err := metricsCollector.EnableFileLogging(logFile); err != nil {
			fmt.Printf("Warning: Failed to enable metrics logging: %v\n", err)
		} else {
			fmt.Printf("Logging metrics to %s\n", logFile)
		}
	}

	// Create output directory if it doesn't exist
	if dir := filepath.Dir(config.MetricsFile); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	// Initialize consumer
	dataConsumer, err := consumer.NewConsumer(config, metricsCollector)
	if err != nil {
		log.Fatalf("Failed to initialize consumer: %v", err)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consumption
	startTime := time.Now()
	fmt.Printf("Starting data consumption at target rate: %d MB/minute\n", config.TargetRate)
	dataConsumer.Start()

	// Print metrics periodically
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Setup metrics saving
	metricsSaveTicker := time.NewTicker(time.Duration(*saveInterval) * time.Second)
	defer metricsSaveTicker.Stop()

	// Initial status message
	fmt.Println("Data consumption started...")
	fmt.Println("Press Ctrl+C to stop")

	// Handle duration or wait for interruption
	var durationTimer *time.Timer
	if config.Duration > 0 {
		fmt.Printf("Will run for %d minutes\n", config.Duration)
		durationTimer = time.NewTimer(time.Duration(config.Duration) * time.Minute)
		defer durationTimer.Stop()
	}

	lastBytes := int64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ticker.C:
			stats := metricsCollector.GetStats()
			now := time.Now()

			// Calculate current rate specifically for this interval
			bytesSinceLastCheck := stats.BytesTransferred - lastBytes
			timeSinceLastCheck := now.Sub(lastTime).Seconds()
			currentRate := float64(0)

			if timeSinceLastCheck > 0 {
				currentRate = float64(bytesSinceLastCheck) / timeSinceLastCheck * 60 / 1024 / 1024
			}

			// Update for next interval
			lastBytes = stats.BytesTransferred
			lastTime = now

			// Print status with improved formatting
			fmt.Printf("\r\033[K") // Clear line
			fmt.Printf("Data: %.2f MB | Rate: %.2f MB/min | Avg: %.2f MB/min | Peak: %.2f MB/min | Time: %s",
				float64(stats.BytesTransferred)/1024/1024,
				currentRate,
				stats.AverageRate,
				stats.PeakRate,
				stats.ElapsedTime.Round(time.Second))

		case <-metricsSaveTicker.C:
			if config.SaveMetrics {
				if err := metricsCollector.SaveStatsToFile(config.MetricsFile); err != nil {
					fmt.Printf("\nWarning: Failed to save metrics: %v\n", err)
				}
			}

		case <-sigChan:
			fmt.Println("\n\nReceived interrupt, shutting down...")
			dataConsumer.Stop()
			saveAndPrintSummary(metricsCollector, config.MetricsFile, startTime)
			return

		case <-func() <-chan time.Time {
			if durationTimer != nil {
				return durationTimer.C
			}
			return make(chan time.Time)
		}():
			fmt.Println("\n\nDuration completed, shutting down...")
			dataConsumer.Stop()
			saveAndPrintSummary(metricsCollector, config.MetricsFile, startTime)
			return
		}
	}
}

func saveAndPrintSummary(m *metrics.Collector, metricsFile string, startTime time.Time) {
	stats := m.GetStats()
	totalRuntime := time.Since(startTime)

	// Save final metrics
	if err := m.SaveStatsToFile(metricsFile); err != nil {
		fmt.Printf("Warning: Failed to save final metrics: %v\n", err)
	} else {
		fmt.Printf("Final metrics saved to %s\n", metricsFile)
	}

	// Print detailed summary
	fmt.Println("\n╔════════════════════════════════════════════╗")
	fmt.Println("║               FINAL SUMMARY                ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Printf("Total data consumed: %.2f MB (%.2f GB)\n",
		stats.TotalMegabytes,
		stats.TotalMegabytes/1024)
	fmt.Printf("Average rate: %.2f MB/min\n", stats.AverageRate)
	fmt.Printf("Peak rate: %.2f MB/min\n", stats.PeakRate)
	fmt.Printf("Last rate: %.2f MB/min\n", stats.CurrentRate)
	fmt.Printf("Total runtime: %s\n", totalRuntime.Round(time.Second))
	fmt.Printf("Started: %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Finished: %s\n", time.Now().Format("2006-01-02 15:04:05"))
}
