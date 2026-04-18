package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

// Metrics handles CSV writing and the live HTTP metrics endpoint.
type Metrics struct {
	eventsSent            atomic.Int64
	eventsComplete        atomic.Int64
	eventsPartial         atomic.Int64
	eventsInFlight        atomic.Int64
	notificationsReceived atomic.Int64

	mu        sync.RWMutex
	latencies []int64 // max latency per completed event, for rolling percentiles

	csvWriter *csv.Writer
	csvFile   *os.File
	pool      *SubscriberPool
}

func NewMetrics(csvPath string, pool *SubscriberPool) (*Metrics, error) {
	f, err := os.Create(csvPath)
	if err != nil {
		return nil, fmt.Errorf("create csv %q: %w", csvPath, err)
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"event_id", "sent_at_unix_ms", "last_received_unix_ms",
		"max_latency_ms", "p50_ms", "p95_ms", "p99_ms",
		"subscribers_received", "subscribers_total", "complete",
	}); err != nil {
		f.Close()
		return nil, err
	}
	w.Flush()
	return &Metrics{csvWriter: w, csvFile: f, pool: pool}, nil
}

func (m *Metrics) IncEventsSent() { m.eventsSent.Add(1) }
func (m *Metrics) IncInFlight()   { m.eventsInFlight.Add(1) }

// RecordResult writes a row to the CSV and updates rolling latency state.
func (m *Metrics) RecordResult(r eventResult) {
	if r.complete {
		m.eventsComplete.Add(1)
	} else {
		m.eventsPartial.Add(1)
	}
	m.eventsInFlight.Add(-1)
	m.notificationsReceived.Add(int64(r.subscribersReceived))

	m.mu.Lock()
	m.latencies = append(m.latencies, r.maxLatencyMS)
	m.mu.Unlock()

	m.csvWriter.Write([]string{
		strconv.FormatInt(r.eventID, 10),
		strconv.FormatInt(r.sentAtMS, 10),
		strconv.FormatInt(r.lastReceivedMS, 10),
		strconv.FormatInt(r.maxLatencyMS, 10),
		strconv.FormatInt(r.p50MS, 10),
		strconv.FormatInt(r.p95MS, 10),
		strconv.FormatInt(r.p99MS, 10),
		strconv.Itoa(r.subscribersReceived),
		strconv.Itoa(r.subscribersTotal),
		strconv.FormatBool(r.complete),
	})
	m.csvWriter.Flush()
}

func (m *Metrics) snapshot() map[string]any {
	m.mu.RLock()
	sorted := make([]int64, len(m.latencies))
	copy(sorted, m.latencies)
	m.mu.RUnlock()

	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var maxLat int64
	if len(sorted) > 0 {
		maxLat = sorted[len(sorted)-1]
	}

	return map[string]any{
		"events_sent":             m.eventsSent.Load(),
		"events_complete":         m.eventsComplete.Load(),
		"events_partial":          m.eventsPartial.Load(),
		"events_in_flight":        m.eventsInFlight.Load(),
		"notifications_received":  m.notificationsReceived.Load(),
		"ws_connections_active":   m.pool.ActiveConns(),
		"latency_p50_ms":          percentile(sorted, 50),
		"latency_p95_ms":          percentile(sorted, 95),
		"latency_p99_ms":          percentile(sorted, 99),
		"latency_max_ms":          maxLat,
	}
}

// ServeHTTP starts the metrics HTTP server. Call in a goroutine.
func (m *Metrics) ServeHTTP(port int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.snapshot())
	})

	mux.HandleFunc("/metrics/prometheus", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		snap := m.snapshot()
		for k, v := range snap {
			fmt.Fprintf(w, "loadtest_%s %v\n", k, v)
		}
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("metrics listening on http://localhost%s/metrics", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("metrics server: %v", err)
	}
}

func (m *Metrics) Close() {
	m.csvWriter.Flush()
	m.csvFile.Close()
}

func (m *Metrics) PrintSummary() {
	snap := m.snapshot()
	fmt.Println("\n=== Load Test Summary ===")
	fmt.Printf("Events sent:            %v\n", snap["events_sent"])
	fmt.Printf("Events complete:        %v\n", snap["events_complete"])
	fmt.Printf("Events partial/timeout: %v\n", snap["events_partial"])
	fmt.Printf("Events in-flight:       %v\n", snap["events_in_flight"])
	fmt.Printf("Notifications received: %v\n", snap["notifications_received"])
	fmt.Printf("Latency p50:            %v ms\n", snap["latency_p50_ms"])
	fmt.Printf("Latency p95:            %v ms\n", snap["latency_p95_ms"])
	fmt.Printf("Latency p99:            %v ms\n", snap["latency_p99_ms"])
	fmt.Printf("Latency max:            %v ms\n", snap["latency_max_ms"])
}
