package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func parseKafkaBrokers() []string {
	raw := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("KAFKA_BROKER"))
	}
	if raw == "" {
		raw = "localhost:9092"
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"localhost:9092"}
	}
	return out
}

func main() {
	brokers := parseKafkaBrokers()

	topic := strings.TrimSpace(os.Getenv("KAFKA_TOPIC"))
	if topic == "" {
		topic = "worker-events"
	}

	producer, err := NewProducer(brokers, topic)
	if err != nil {
		log.Fatalf("failed to create kafka producer: %v", err)
	}
	defer producer.Close()

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://notif:notif@localhost:5432/notifications?sslmode=disable"
	}
	followerDB, err := NewFollowerDB(dsn)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer followerDB.Close()

	mode := strings.TrimSpace(os.Getenv("NOTIFICATION_MODE"))
	if mode == "" {
		mode = "FAN_OUT_HYBRID"
	}

	threshold := 1000
	if t := strings.TrimSpace(os.Getenv("FANOUT_THRESHOLD")); t != "" {
		if v, err := strconv.Atoi(t); err == nil {
			threshold = v
		}
	}

	h := &Handler{
		producer:  producer,
		db:        followerDB,
		mode:      mode,
		threshold: threshold,
	}

	http.HandleFunc("/event", h.HandleEvent)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("ingestion API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
