package util

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	DefaultNatsURL   = nats.DefaultURL
	StreamName       = "WEBHOOK_STREAM"
	StreamSubject    = "webhook.messages"
	DurableConsumer  = "webhook-deliverer"
	DeliveryMaxRetry = 4
	AckWait          = 30 * time.Second
)

var Backoff = []time.Duration{
	2 * time.Second,
	8 * time.Second,
	32 * time.Second,
	64 * time.Second,
}

func Connect() (*nats.Conn, nats.JetStreamContext) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = DefaultNatsURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("NATS connection error: %v", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("JetStream init error: %v", err)
	}

	// Create stream if needed
	ensureStream(js, &nats.StreamConfig{
		Name:     StreamName,
		Subjects: []string{StreamSubject},
	})

	return nc, js
}

func ensureStream(js nats.JetStreamContext, cfg *nats.StreamConfig) {
	_, err := js.StreamInfo(cfg.Name)
	if err == nil {
		log.Printf("stream %s already exists", cfg.Name)
		return
	}
	if !errors.Is(err, nats.ErrStreamNotFound) {
		log.Fatalf("stream info check failed: %v", err)
	}
	if _, err := js.AddStream(cfg); err != nil {
		log.Fatalf("failed to create stream %s: %v", cfg.Name, err)
	}
	log.Printf("stream %s created", cfg.Name)
}

func EnsureConsumer(js nats.JetStreamContext) {
	cfg := &nats.ConsumerConfig{
		Durable:       DurableConsumer,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       AckWait,          // 30s
		MaxDeliver:    DeliveryMaxRetry, // 4
		BackOff:       Backoff,          // []time.Duration{...}
		DeliverPolicy: nats.DeliverNewPolicy,
		FilterSubject: StreamSubject,
	}

	if _, err := js.UpdateConsumer(StreamName, cfg); err != nil {
		if errors.Is(err, nats.ErrConsumerNotFound) {
			if _, err := js.AddConsumer(StreamName, cfg); err != nil {
				log.Fatalf("add consumer failed: %v", err)
			}
		} else {
			log.Fatalf("update consumer failed: %v", err)
		}
	}
}
