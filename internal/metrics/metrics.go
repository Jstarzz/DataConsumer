package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Stats represents data usage statistics
type Stats struct {
	BytesTransferred int64
	ElapsedTime      time.Duration
	StartTime        time.Time
	CurrentRate      float64 // MB per minute
	PeakRate         float64 // Peak MB per minute
	AverageRate      float64 // Average MB per minute
	TotalMegabytes   float64 // Total MB transferred
	RateHistory      []RatePoint
	LastUpdated      time.Time
}

// RatePoint represents a rate measurement at a specific time
type RatePoint struct {
	Timestamp time.Time
	RateMBPS  float64
}

// Collector tracks data usage metrics
type Collector struct {
	bytesTransferred int64
	startTime        time.Time
	lastSample       time.Time
	lastBytes        int64
	running          bool
	peakRate         float64
	rateHistory      []RatePoint
	historyLimit     int
	mu               sync.Mutex
	logFile          *os.File
	enableLogging    bool
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		historyLimit:  60, // Keep one hour of minute-by-minute history
		enableLogging: false,
	}
}

// EnableFileLogging enables logging metrics to a file
func (m *Collector) EnableFileLogging(filename string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	m.logFile = file
	m.enableLogging = true

	// Write CSV header
	_, err = file.WriteString("timestamp,bytes_transferred,rate_mbps,total_mb\n")
	return err
}

// Start initializes the metrics collection
func (m *Collector) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		now := time.Now()
		m.startTime = now
		m.lastSample = now
		atomic.StoreInt64(&m.bytesTransferred, 0)
		m.lastBytes = 0
		m.peakRate = 0
		m.rateHistory = make([]RatePoint, 0, m.historyLimit)
		m.running = true

		// Start metrics sampling in background
		go m.sampleMetrics()
	}
}

// sampleMetrics periodically samples the current rate
func (m *Collector) sampleMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		if !m.running {
			m.mu.Unlock()
			return
		}

		// Calculate current rate
		now := time.Now()
		currentBytes := atomic.LoadInt64(&m.bytesTransferred)
		bytesDelta := currentBytes - m.lastBytes
		timeDelta := now.Sub(m.lastSample).Seconds()

		if timeDelta > 0 {
			// Calculate rate in MB/s
			rateMBPS := float64(bytesDelta) / timeDelta / 1024 / 1024

			// Record to history (convert to MB/min for consistency)
			if len(m.rateHistory) >= m.historyLimit {
				// Remove oldest entry
				m.rateHistory = m.rateHistory[1:]
			}

			m.rateHistory = append(m.rateHistory, RatePoint{
				Timestamp: now,
				RateMBPS:  rateMBPS * 60, // Convert to MB/min
			})

			// Update peak rate
			if rateMBPS*60 > m.peakRate {
				m.peakRate = rateMBPS * 60
			}

			// Update last sample values
			m.lastSample = now
			m.lastBytes = currentBytes

			// Log to file if enabled
			if m.enableLogging && m.logFile != nil {
				totalMB := float64(currentBytes) / 1024 / 1024
				logLine := fmt.Sprintf("%s,%d,%.2f,%.2f\n",
					now.Format(time.RFC3339),
					currentBytes,
					rateMBPS,
					totalMB)
				m.logFile.WriteString(logLine)
			}
		}

		m.mu.Unlock()
	}
}

// Stop finalizes the metrics collection
func (m *Collector) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.running = false

	// Close log file if open
	if m.logFile != nil {
		m.logFile.Close()
		m.logFile = nil
	}
}

// AddBytes records additional bytes transferred
func (m *Collector) AddBytes(bytes int64) {
	atomic.AddInt64(&m.bytesTransferred, bytes)
}

// GetStats returns the current statistics
func (m *Collector) GetStats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentBytes := atomic.LoadInt64(&m.bytesTransferred)
	elapsed := time.Since(m.startTime)

	// Calculate current rate (MB/min)
	var currentRate float64
	if len(m.rateHistory) > 0 {
		currentRate = m.rateHistory[len(m.rateHistory)-1].RateMBPS
	} else if elapsed.Seconds() > 0 {
		currentRate = float64(currentBytes) / elapsed.Seconds() * 60 / 1024 / 1024
	}

	// Calculate average rate
	averageRate := float64(0)
	if elapsed.Minutes() > 0 {
		averageRate = float64(currentBytes) / 1024 / 1024 / elapsed.Minutes()
	}

	return Stats{
		BytesTransferred: currentBytes,
		ElapsedTime:      elapsed,
		StartTime:        m.startTime,
		CurrentRate:      currentRate,
		PeakRate:         m.peakRate,
		AverageRate:      averageRate,
		TotalMegabytes:   float64(currentBytes) / 1024 / 1024,
		RateHistory:      m.rateHistory,
		LastUpdated:      time.Now(),
	}
}

// SaveStatsToFile saves the current stats to a JSON file
func (m *Collector) SaveStatsToFile(filename string) error {
	stats := m.GetStats()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(stats)
}
