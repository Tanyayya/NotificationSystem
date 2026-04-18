package main

import (
	"log"
	"net/http"
	"os"
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
	if err != nil {
		log.Fatalf("failed to create kafka producer: %v", err)
	}
	defer producer.Close()

	// Creates your handler  and gives it the Kafka producer.
	h := &Handler{producer: producer}

	http.HandleFunc("/event", h.HandleEvent)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("ingestion API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}