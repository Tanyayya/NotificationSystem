package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tanyayya/NotificationSystem/fanout/internal/config"
	"github.com/Tanyayya/NotificationSystem/fanout/internal/consumer"
	"github.com/Tanyayya/NotificationSystem/fanout/internal/notif"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	pub := notif.NewPublisher(cfg.RedisAddr, cfg.NotifyType, cfg.NotifyFromUser, cfg.NotifyMessage)
	defer func() {
		if err := pub.Close(); err != nil {
			log.Printf("redis close: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("http listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	// Run reads messages from the given topic until the context is cancelled.
	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Run(ctx, cfg.Brokers, cfg.Topic, cfg.GroupID, cfg.NotifyDefaultUserID, pub)
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	if err := <-consumerDone; err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("consumer stopped with error: %v", err)
		os.Exit(1)
	}
	log.Println("shutdown complete")
}
