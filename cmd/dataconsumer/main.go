package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"dataconsumer/configs"
	"dataconsumer/internal/consumer"
	"dataconsumer/internal/metrics"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	duration := flag.Int("duration", 0, "Duration to run in minutes (0 for indefinite)")
	outputMetrics := flag.String("metrics", "dataconsumer_metrics.json", "Path to save metrics")
	saveInterval := flag.Int("save-interval", 60, "Save metrics every N seconds")
	flag.Parse()

	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║                 DATA CONSUMER v2.0                 ║")
	fmt.Println("║      High-Performance Network Data Consumer      ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Printf("Running on %s with %d CPU cores\n\n", runtime.GOOS, runtime.NumCPU())

	config := configs.DefaultConfig()
	if *configPath != "" {
		var err error
		config, err = configs.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}
	}

	var targetRateInput string
	defaultRate := config.TargetRate
	fmt.Printf("Enter target data consumption rate in MB/min (default: %d, or press Enter for default): ", defaultRate)
	fmt.Scanln(&targetRateInput)
	if targetRateInput != "" {
		rate, err := strconv.Atoi(targetRateInput)
		if err != nil {
			fmt.Printf("Invalid target rate '%s'. Using default: %d MB/min.\n", targetRateInput, defaultRate)
		} else {
			config.TargetRate = rate
		}
	} else {
		fmt.Printf("Using default target rate: %d MB/min.\n", defaultRate)
	}

	var verboseInput string
	defaultVerbose := "N"
	if config.VerboseLogging {
		defaultVerbose = "Y"
	}
	fmt.Printf("Enable verbose logging? (y/N, default: %s, or press Enter for default): ", defaultVerbose)
	fmt.Scanln(&verboseInput)
	if verboseInput == "y" || verboseInput == "Y" {
		config.VerboseLogging = true
	} else if verboseInput != "" && verboseInput != "n" && verboseInput != "N" {
		fmt.Printf("Invalid input '%s'. Using default: %s.\n", verboseInput, defaultVerbose)
		config.VerboseLogging = defaultVerbose == "Y"
	} else {
		fmt.Printf("Using default verbose logging: %s.\n", defaultVerbose)
	}

	var workersInput string
	defaultWorkers := runtime.NumCPU()
	fmt.Printf("Enter the number of workers to use (default: %d, or press Enter for default): ", defaultWorkers)
	fmt.Scanln(&workersInput)
	if workersInput != "" {
		workers, err := strconv.Atoi(workersInput)
		if err != nil {
			fmt.Printf("Invalid number of workers '%s'. Using default: %d.\n", workersInput, defaultWorkers)
		} else {
			config.ConcurrencyFactor = workers
		}
	} else {
		config.ConcurrencyFactor = defaultWorkers
		fmt.Printf("Using default number of workers: %d.\n", defaultWorkers)
	}

	config.Duration = *duration
	config.MetricsFile = *outputMetrics

	metricsCollector := metrics.NewCollector()

	if config.SaveMetrics {
		logFile := fmt.Sprintf("dataconsumer_log_%s.csv", time.Now().Format("20060102_150405"))
		if err := metricsCollector.EnableFileLogging(logFile); err != nil {
			fmt.Printf("Warning: Failed to enable metrics logging: %v\n", err)
		} else {
			fmt.Printf("Logging metrics to %s\n", logFile)
		}
	}

	os.MkdirAll(filepath.Dir(config.MetricsFile), 0755)

	dataConsumer, err := consumer.NewConsumer(config, metricsCollector)
	if err != nil {
		log.Fatalf("Failed to initialize consumer: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	startTime := time.Now()
	fmt.Printf("Starting data consumption targeting at least %d MB/minute\n", config.TargetRate)
	dataConsumer.Start()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	metricsSaveTicker := time.NewTicker(time.Duration(*saveInterval) * time.Second)
	defer metricsSaveTicker.Stop()

	fmt.Println("Data consumption started...")
	fmt.Println("Press Ctrl+C to stop")

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
			bytesSinceLast := stats.BytesTransferred - lastBytes
			timeSinceLast := now.Sub(lastTime).Seconds()
			currentRate := float64(0)
			if timeSinceLast > 0 {
				currentRate = float64(bytesSinceLast) / timeSinceLast * 60 / 1024 / 1024
			}
			lastBytes = stats.BytesTransferred
			lastTime = now

			fmt.Printf("\r\033[KData: %.2f MB | Rate: %.2f MB/min | Avg: %.2f MB/min | Peak: %.2f MB/min | Time: %s",
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

	if err := m.SaveStatsToFile(metricsFile); err != nil {
		fmt.Printf("Warning: Failed to save final metrics: %v\n", err)
	} else {
		fmt.Printf("Final metrics saved to %s\n", metricsFile)
	}

	fmt.Println("\n╔════════════════════════════════════════════╗")
	fmt.Println("║                   FINAL SUMMARY                  ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Printf("Total data consumed: %.2f MB (%.2f GB)\n", stats.TotalMegabytes, stats.TotalMegabytes/1024)
	fmt.Printf("Average rate: %.2f MB/min\n", stats.AverageRate)
	fmt.Printf("Peak rate: %.2f MB/min\n", stats.PeakRate)
	fmt.Printf("Last rate: %.2f MB/min\n", stats.CurrentRate)
	fmt.Printf("Total runtime: %s\n", totalRuntime.Round(time.Second))
}
