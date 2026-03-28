package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
)

func main() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("redis close: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pubsub := rdb.PSubscribe(ctx, "notif:*")
	defer func() {
		if err := pubsub.Close(); err != nil {
			log.Printf("pubsub close: %v", err)
		}
	}()

	log.Printf("redis subscriber PSUBSCRIBE notif:* addr=%s", addr)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown signal received")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg == nil {
				continue
			}
			log.Printf("redis message channel=%q payload=%s", msg.Channel, msg.Payload)
		}
	}
}
