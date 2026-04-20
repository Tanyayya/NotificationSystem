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
	"time"
)

// Metrics handles CSV writing and the live HTTP metrics endpoint.
type Metrics struct {
	eventsSent            atomic.Int64
	eventsComplete        atomic.Int64
	eventsPartial         atomic.Int64
	eventsInFlight        atomic.Int64
	notificationsReceived atomic.Int64
	historiesReceived     atomic.Int64

	mu               sync.RWMutex
	latencies        []int64 // max latency per completed event, for rolling percentiles
	historyLatencies []int64 // connect-to-first-message latency per connection

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
		"record_type", "event_id", "sent_at_unix_ms", "user_id", "received_at_unix_ms", "latency_ms",
	}); err != nil {
		f.Close()
		return nil, err
	}
	w.Flush()
	return &Metrics{csvWriter: w, csvFile: f, pool: pool}, nil
}

func (m *Metrics) IncInFlight()      { m.eventsInFlight.Add(1) }
func (m *Metrics) InFlight() int64  { return m.eventsInFlight.Load() }

// RecordEventSent writes a "sent" row to the CSV and increments the sent counter.
func (m *Metrics) RecordEventSent(eventID int64, sentAt time.Time) {
	m.eventsSent.Add(1)
	m.mu.Lock()
	m.csvWriter.Write([]string{
		"sent",
		strconv.FormatInt(eventID, 10),
		strconv.FormatInt(sentAt.UnixMilli(), 10),
		"", "", "",
	})
	m.csvWriter.Flush()
	m.mu.Unlock()
}

// RecordNotificationReceived writes a "received" row to the CSV and updates rolling latency state.
func (m *Metrics) RecordNotificationReceived(eventID int64, userID string, sentAt time.Time, receivedAt time.Time) {
	latencyMS := receivedAt.Sub(sentAt).Milliseconds()
	m.notificationsReceived.Add(1)
	m.mu.Lock()
	m.latencies = append(m.latencies, latencyMS)
	m.csvWriter.Write([]string{
		"received",
		strconv.FormatInt(eventID, 10),
		strconv.FormatInt(sentAt.UnixMilli(), 10),
		userID,
		strconv.FormatInt(receivedAt.UnixMilli(), 10),
		strconv.FormatInt(latencyMS, 10),
	})
	m.csvWriter.Flush()
	m.mu.Unlock()
}

// RecordHistoryReceived writes a "history_received" row to the CSV and records
// the latency from WebSocket connection established to first history message.
func (m *Metrics) RecordHistoryReceived(userID string, connectedAt time.Time, receivedAt time.Time) {
	latencyMS := receivedAt.Sub(connectedAt).Milliseconds()
	m.historiesReceived.Add(1)
	m.mu.Lock()
	m.historyLatencies = append(m.historyLatencies, latencyMS)
	m.csvWriter.Write([]string{
		"history_received",
		"",
		strconv.FormatInt(connectedAt.UnixMilli(), 10),
		userID,
		strconv.FormatInt(receivedAt.UnixMilli(), 10),
		strconv.FormatInt(latencyMS, 10),
	})
	m.csvWriter.Flush()
	m.mu.Unlock()
}

// RecordNotDelivered writes a "not_delivered" row to the CSV for a subscriber that
// was still connected but did not receive the notification within the 2-second window.
func (m *Metrics) RecordNotDelivered(eventID int64, userID string, sentAt time.Time) {
	m.mu.Lock()
	m.csvWriter.Write([]string{
		"not_delivered",
		strconv.FormatInt(eventID, 10),
		strconv.FormatInt(sentAt.UnixMilli(), 10),
		userID,
		"",
		"",
	})
	m.csvWriter.Flush()
	m.mu.Unlock()
}

// RecordResult updates event completion counters.
func (m *Metrics) RecordResult(r eventResult) {
	if r.complete {
		m.eventsComplete.Add(1)
	} else {
		m.eventsPartial.Add(1)
	}
	m.eventsInFlight.Add(-1)
}

func (m *Metrics) snapshot() map[string]any {
	m.mu.RLock()
	sorted := make([]int64, len(m.latencies))
	copy(sorted, m.latencies)
	historySorted := make([]int64, len(m.historyLatencies))
	copy(historySorted, m.historyLatencies)
	m.mu.RUnlock()

	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	sort.Slice(historySorted, func(i, j int) bool { return historySorted[i] < historySorted[j] })

	var maxLat int64
	if len(sorted) > 0 {
		maxLat = sorted[len(sorted)-1]
	}
	var maxHistoryLat int64
	if len(historySorted) > 0 {
		maxHistoryLat = historySorted[len(historySorted)-1]
	}

	return map[string]any{
		"events_sent":              m.eventsSent.Load(),
		"events_complete":          m.eventsComplete.Load(),
		"events_partial":           m.eventsPartial.Load(),
		"events_in_flight":         m.eventsInFlight.Load(),
		"notifications_received":   m.notificationsReceived.Load(),
		"histories_received":       m.historiesReceived.Load(),
		"ws_connections_active":    m.pool.ActiveConns(),
		"latency_p50_ms":           percentile(sorted, 50),
		"latency_p95_ms":           percentile(sorted, 95),
		"latency_p99_ms":           percentile(sorted, 99),
		"latency_max_ms":           maxLat,
		"history_latency_p50_ms":   percentile(historySorted, 50),
		"history_latency_p95_ms":   percentile(historySorted, 95),
		"history_latency_p99_ms":   percentile(historySorted, 99),
		"history_latency_max_ms":   maxHistoryLat,
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

func (m *Metrics) PrintSummary(phase string) {
	snap := m.snapshot()
	fmt.Println("\n=== Load Test Summary ===")
	if phase == "1" || phase == "both" {
		fmt.Printf("Histories received:       %v\n", snap["histories_received"])
		fmt.Printf("History latency p50:      %v ms\n", snap["history_latency_p50_ms"])
		fmt.Printf("History latency p95:      %v ms\n", snap["history_latency_p95_ms"])
		fmt.Printf("History latency p99:      %v ms\n", snap["history_latency_p99_ms"])
		fmt.Printf("History latency max:      %v ms\n", snap["history_latency_max_ms"])
	}
	if phase == "2" || phase == "both" {
		if phase == "both" {
			fmt.Println()
		}
		fmt.Printf("Events sent:              %v\n", snap["events_sent"])
		fmt.Printf("Events complete:          %v\n", snap["events_complete"])
		fmt.Printf("Events partial/timeout:   %v\n", snap["events_partial"])
		fmt.Printf("Events in-flight:         %v\n", snap["events_in_flight"])
		fmt.Printf("Notifications received:   %v\n", snap["notifications_received"])
		fmt.Printf("Notification latency p50: %v ms\n", snap["latency_p50_ms"])
		fmt.Printf("Notification latency p95: %v ms\n", snap["latency_p95_ms"])
		fmt.Printf("Notification latency p99: %v ms\n", snap["latency_p99_ms"])
		fmt.Printf("Notification latency max: %v ms\n", snap["latency_max_ms"])
	}
}
