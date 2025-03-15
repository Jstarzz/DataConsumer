package consumer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"dataconsumer/configs"
	"dataconsumer/internal/metrics"
)

// countingDiscarder counts bytes and discards them
type countingDiscarder struct {
	collector *metrics.Collector
}

func (w *countingDiscarder) Write(p []byte) (n int, err error) {
	n = len(p)
	w.collector.AddBytes(int64(n))
	return n, nil
}

type Consumer struct {
	config           *configs.Config
	metricsCollector *metrics.Collector
	client           *http.Client
	cancel           context.CancelFunc
	ctx              context.Context
	wg               sync.WaitGroup
}

func NewConsumer(config *configs.Config, metricsCollector *metrics.Collector) (*Consumer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	transport := &http.Transport{
		MaxIdleConns:          200,
		MaxConnsPerHost:       200,
		MaxIdleConnsPerHost:   200,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DisableCompression:    true,
	}
	client := &http.Client{Transport: transport}

	return &Consumer{
		config:           config,
		metricsCollector: metricsCollector,
		client:           client,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

func (c *Consumer) Start() {
	c.metricsCollector.Start()
	numWorkers := 150 // Increased for higher throughput
	if c.config.VerboseLogging {
		fmt.Printf("Starting %d workers to achieve at least %d MB/minute\n", numWorkers, c.config.TargetRate)
	}
	for i := 0; i < numWorkers; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}
}

func (c *Consumer) Stop() {
	c.cancel()
	c.wg.Wait()
	c.metricsCollector.Stop()
}

func (c *Consumer) worker(id int) {
	defer c.wg.Done()
	sources := c.config.DataSources
	sourceIndex := id % len(sources)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			for attempt := 0; attempt < 3; attempt++ { // Retry up to 3 times
				if c.consumeData(sources[sourceIndex]) {
					break // Success, move to next source
				}
				if c.config.VerboseLogging {
					fmt.Printf("Retrying %s (attempt %d)\n", sources[sourceIndex], attempt+1)
				}
				time.Sleep(500 * time.Millisecond) // Brief pause before retry
			}
			sourceIndex = (sourceIndex + 1) % len(sources)
		}
	}
}

func (c *Consumer) consumeData(url string) bool {
	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")
	if c.config.UseRandomization {
		req.URL.RawQuery = fmt.Sprintf("t=%d", time.Now().UnixNano())
	}

	resp, err := c.client.Do(req)
	if err != nil {
		if c.config.VerboseLogging {
			fmt.Printf("Error downloading from %s: %v\n", url, err)
		}
		return false
	}
	defer resp.Body.Close()

	buffer := make([]byte, 2097152) // 2 MB buffer
	discarder := &countingDiscarder{collector: c.metricsCollector}
	_, err = io.CopyBuffer(discarder, resp.Body, buffer)
	if err != nil && err != context.Canceled {
		if c.config.VerboseLogging {
			fmt.Printf("Error downloading from %s: %v\n", url, err)
		}
		return false
	}
	return true
}
