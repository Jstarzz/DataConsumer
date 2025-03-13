package consumer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"dataconsumer/configs"
	"dataconsumer/internal/metrics"
)

// Consumer handles the data consumption process
type Consumer struct {
	config           *configs.Config
	metricsCollector *metrics.Collector
	client           *http.Client
	cancel           context.CancelFunc
	ctx              context.Context
	wg               sync.WaitGroup
	rateLimiter      *RateLimiter
}

// RateLimiter controls data consumption rate
type RateLimiter struct {
	targetBytesPerSecond int64
	mu                   sync.Mutex
	lastCheck            time.Time
	bytesThisPeriod      int64
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(targetMBPerMinute int) *RateLimiter {
	// Convert MB/minute to bytes/second
	bytesPerSecond := int64(targetMBPerMinute) * 1024 * 1024 / 60
	return &RateLimiter{
		targetBytesPerSecond: bytesPerSecond,
		lastCheck:            time.Now(),
	}
}

// Allow checks if more data can be consumed and sleeps if necessary
func (r *RateLimiter) Allow(bytes int64) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastCheck)

	// Reset counter if more than a second has passed
	if elapsed > time.Second {
		r.bytesThisPeriod = 0
		r.lastCheck = now
		return 0
	}

	r.bytesThisPeriod += bytes
	allowedBytes := int64(float64(r.targetBytesPerSecond) * elapsed.Seconds())

	// If we've consumed more than allowed for this period, sleep
	if r.bytesThisPeriod > allowedBytes {
		// Calculate how long to sleep
		excessBytes := r.bytesThisPeriod - allowedBytes
		sleepTime := time.Duration(float64(excessBytes) / float64(r.targetBytesPerSecond) * float64(time.Second))

		// Cap maximum sleep time to prevent long pauses
		if sleepTime > 500*time.Millisecond {
			sleepTime = 500 * time.Millisecond
		}

		return sleepTime
	}

	return 0
}

// NewConsumer creates a new data consumer
func NewConsumer(config *configs.Config, metricsCollector *metrics.Collector) (*Consumer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure optimized HTTP client
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxConnsPerHost:     200,
		MaxIdleConnsPerHost: 200,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true, // Faster for our purposes
	}

	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}

	// Create rate limiter for target consumption rate
	rateLimiter := NewRateLimiter(config.TargetRate)

	return &Consumer{
		config:           config,
		metricsCollector: metricsCollector,
		client:           client,
		ctx:              ctx,
		cancel:           cancel,
		rateLimiter:      rateLimiter,
	}, nil
}

// Start begins the data consumption process
func (c *Consumer) Start() {
	// Start metrics collector
	c.metricsCollector.Start()

	// Calculate optimal number of workers based on CPU cores and target rate
	numWorkers := runtime.NumCPU() * 2
	if c.config.TargetRate > 200 {
		// For very high rates, increase workers
		numWorkers = runtime.NumCPU() * 4
	}

	if c.config.VerboseLogging {
		fmt.Printf("Starting %d workers to achieve %d MB/minute\n", numWorkers, c.config.TargetRate)
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}
}

// Stop halts the data consumption process
func (c *Consumer) Stop() {
	c.cancel()
	c.wg.Wait()
	c.metricsCollector.Stop()
}

// worker continuously downloads data
func (c *Consumer) worker(id int) {
	defer c.wg.Done()

	// Randomize URLs to avoid CDN caching
	sources := c.getDataSources()
	sourceIndex := id % len(sources)

	// Use different buffer sizes for different workers
	// This helps distribute the load and avoid synchronization
	bufferSizes := []int{4096, 8192, 16384, 32768}
	bufferSize := bufferSizes[id%len(bufferSizes)]
	buffer := make([]byte, bufferSize)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.consumeData(sources[sourceIndex], buffer)

			// Rotate through available sources
			sourceIndex = (sourceIndex + 1) % len(sources)
		}
	}
}

// getDataSources returns a list of URLs to download from
func (c *Consumer) getDataSources() []string {
	// Use multiple sources to distribute load and avoid rate limiting
	return []string{
		"https://speed.cloudflare.com/10mb.bin",
		"https://speed.cloudflare.com/100mb.bin",
		"https://cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js",      // Popular CDN file
		"https://ajax.googleapis.com/ajax/libs/jquery/3.6.0/jquery.min.js",  // Google CDN
		"https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.0/jquery.min.js", // Cloudflare CDN
	}
}

// consumeData performs a single data consumption operation
func (c *Consumer) consumeData(url string, buffer []byte) {
	// Create request with appropriate headers
	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return
	}

	// Add headers to make the request more realistic
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")

	// Add a random query parameter to avoid caching
	timestamp := time.Now().UnixNano()
	req.URL.RawQuery = fmt.Sprintf("t=%d", timestamp)

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		if c.config.VerboseLogging {
			fmt.Printf("Error downloading from %s: %v\n", url, err)
		}
		time.Sleep(100 * time.Millisecond) // Brief pause on error
		return
	}
	defer resp.Body.Close()

	// Read and discard data while counting bytes
	totalRead := int64(0)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			n, err := resp.Body.Read(buffer)

			if n > 0 {
				totalRead += int64(n)
				c.metricsCollector.AddBytes(int64(n))

				// Rate limiting
				if sleepTime := c.rateLimiter.Allow(int64(n)); sleepTime > 0 {
					time.Sleep(sleepTime)
				}
			}

			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
		}
	}
}
