package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "kafka:9092"
	}

	producer, err := NewProducer(broker)
	if err != nil {
		log.Fatalf("failed to create kafka producer: %v", err)
	}
	defer producer.Close()

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