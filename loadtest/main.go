package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := parseConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT / SIGTERM for graceful shutdown.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	tracker := NewTracker(cfg.FollowerCount, cfg.EventTimeout)
	pool := NewSubscriberPool(cfg.ALBURL, cfg.FollowerStart, cfg.FollowerCount, tracker)

	metrics, err := NewMetrics(cfg.CSVOut, pool)
	if err != nil {
		log.Fatalf("metrics init: %v", err)
	}
	defer metrics.Close()

	go metrics.ServeHTTP(cfg.MetricsPort)

	// Drain tracker results and record them.
	go func() {
		for result := range tracker.Results {
			metrics.RecordResult(result)
		}
	}()

	// Process delivery reports sequentially.
	go tracker.Run(ctx)

	// Open all WebSocket connections before publishing events.
	ready := make(chan struct{})
	pool.Start(ctx, ready)

	log.Printf("waiting for %d subscriber connections...", cfg.FollowerCount)
	select {
	case <-ready:
		log.Printf("all %d subscribers connected, starting publisher", cfg.FollowerCount)
	case <-time.After(2 * time.Minute):
		log.Printf("timed out waiting for all subscribers (%d/%d active), proceeding anyway",
			pool.ActiveConns(), cfg.FollowerCount)
	case <-sigs:
		log.Println("interrupted before test started")
		return
	}

	testCtx, testCancel := context.WithTimeout(ctx, cfg.Duration)
	defer testCancel()

	publisher := NewPublisher(cfg.IngestionURL, cfg.SenderUser, cfg.EventRate, tracker, metrics)
	go publisher.Run(testCtx)

	// Block until test duration elapses or the user interrupts.
	select {
	case <-testCtx.Done():
		log.Println("test duration elapsed, waiting for in-flight events...")
	case <-sigs:
		log.Println("interrupted, waiting for in-flight events...")
		cancel()
	}

	// Give in-flight events time to complete or time out before printing results.
	time.Sleep(cfg.EventTimeout)

	metrics.PrintSummary()
	log.Printf("results written to %s", cfg.CSVOut)
}
