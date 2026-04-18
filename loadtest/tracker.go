package main

import (
	"context"
	"sort"
	"sync"
	"time"
)

// report is sent by a subscriber goroutine when it receives a notification.
type report struct {
	eventID int64
	userID  string
	at      time.Time
}

// eventResult is emitted by the tracker when an event is complete or timed out.
type eventResult struct {
	eventID             int64
	sentAtMS            int64
	lastReceivedMS      int64
	maxLatencyMS        int64
	p50MS               int64
	p95MS               int64
	p99MS               int64
	subscribersReceived int
	subscribersTotal    int
	complete            bool
}

type inflightEvent struct {
	eventID       int64
	sentAt        time.Time
	mu            sync.Mutex
	received      map[string]time.Time
	totalExpected int
	timer         *time.Timer
}

// Tracker matches subscriber delivery reports to sent events and computes latency.
type Tracker struct {
	events        sync.Map
	Reports       chan report
	Results       chan eventResult
	totalExpected int
	eventTimeout  time.Duration
}

func NewTracker(totalExpected int, eventTimeout time.Duration) *Tracker {
	return &Tracker{
		Reports:       make(chan report, totalExpected*4),
		Results:       make(chan eventResult, 256),
		totalExpected: totalExpected,
		eventTimeout:  eventTimeout,
	}
}

// Register records a new in-flight event. Call immediately after POST /event returns.
func (t *Tracker) Register(eventID int64, sentAt time.Time) {
	ev := &inflightEvent{
		eventID:       eventID,
		sentAt:        sentAt,
		received:      make(map[string]time.Time, t.totalExpected),
		totalExpected: t.totalExpected,
	}
	t.events.Store(eventID, ev)
	ev.timer = time.AfterFunc(t.eventTimeout, func() {
		t.finalize(eventID, false)
	})
}

// Run drains the Reports channel and processes delivery confirmations.
// Must be called in its own goroutine.
func (t *Tracker) Run(ctx context.Context) {
	for {
		select {
		case r := <-t.Reports:
			t.handleReport(r)
		case <-ctx.Done():
			return
		}
	}
}

func (t *Tracker) handleReport(r report) {
	val, ok := t.events.Load(r.eventID)
	if !ok {
		return
	}
	ev := val.(*inflightEvent)

	ev.mu.Lock()
	if _, already := ev.received[r.userID]; !already {
		ev.received[r.userID] = r.at
	}
	complete := len(ev.received) >= ev.totalExpected
	ev.mu.Unlock()

	if complete {
		ev.timer.Stop()
		t.finalize(r.eventID, true)
	}
}

// finalize is safe to call from multiple goroutines; sync.Map.LoadAndDelete ensures
// only the first call proceeds.
func (t *Tracker) finalize(eventID int64, complete bool) {
	val, loaded := t.events.LoadAndDelete(eventID)
	if !loaded {
		return
	}
	ev := val.(*inflightEvent)

	ev.mu.Lock()
	defer ev.mu.Unlock()

	latencies := make([]int64, 0, len(ev.received))
	var lastReceived time.Time
	for _, at := range ev.received {
		d := at.Sub(ev.sentAt).Milliseconds()
		latencies = append(latencies, d)
		if at.After(lastReceived) {
			lastReceived = at
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	result := eventResult{
		eventID:             eventID,
		sentAtMS:            ev.sentAt.UnixMilli(),
		subscribersReceived: len(ev.received),
		subscribersTotal:    ev.totalExpected,
		complete:            complete,
	}
	if !lastReceived.IsZero() {
		result.lastReceivedMS = lastReceived.UnixMilli()
	}
	if len(latencies) > 0 {
		result.maxLatencyMS = latencies[len(latencies)-1]
		result.p50MS = percentile(latencies, 50)
		result.p95MS = percentile(latencies, 95)
		result.p99MS = percentile(latencies, 99)
	}

	select {
	case t.Results <- result:
	default:
		// Results channel full — drop rather than block; metrics will show events_in_flight not draining
	}
}

func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
