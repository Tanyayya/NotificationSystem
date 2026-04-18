package main

import (
	"log"
	"net/http"
	"os"
<<<<<<< HEAD
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
=======
)

func main() {
	// Reads the Kafka address from the environment. 
	// If nothing is set, it defaults to kafka:9092 — which is the address Docker Compose uses internally.
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "kafka:9092"
	}

	// creates the kafka connection 
	producer, err := NewProducer(broker)
>>>>>>> ts-notifications-read-api
	if err != nil {
		log.Fatalf("failed to create kafka producer: %v", err)
	}
	defer producer.Close()

<<<<<<< HEAD
=======
	// Creates your handler  and gives it the Kafka producer.
>>>>>>> ts-notifications-read-api
	h := &Handler{producer: producer}

	http.HandleFunc("/event", h.HandleEvent)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
<<<<<<< HEAD
		w.Write([]byte("{'status': 'ok'}"))
=======
>>>>>>> ts-notifications-read-api
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("ingestion API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}