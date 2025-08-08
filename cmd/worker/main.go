package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/matthewvanderw/chirp-webhook-server/internal/message"
	"github.com/matthewvanderw/chirp-webhook-server/internal/util"
	"github.com/nats-io/nats.go"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func sendWebhook(dest message.DestConfig, body []byte) error {

	req, err := http.NewRequest("POST", dest.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if dest.AuthType != "none" {
		switch dest.AuthType {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+dest.AuthValue)
		case "basic":
			req.Header.Set("Authorization", "Basic "+dest.AuthValue)
		default:
			req.Header.Set("X-Webhook-Auth-Type", dest.AuthType)
			req.Header.Set("X-Webhook-Auth-Value", dest.AuthValue)
			req.Header.Set(dest.AuthType, dest.AuthValue)
		}
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		return nats.ErrNoResponders // trigger redelivery
	}

	return nil
}

func main() {
	nc, js := util.Connect()
	util.EnsureConsumer(js)
	defer nc.Drain()

	// Setup durable consumer with JetStream-managed retry
	_, err := js.Subscribe(util.StreamSubject, func(m *nats.Msg) {
		var msg message.WebhookMessage
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			log.Printf("invalid message: %v", err)
			m.Term()
			return
		}

		log.Printf("Delivering to %s", msg.Dest.URL)
		if err := sendWebhook(msg.Dest, msg.Body); err != nil {
			log.Printf("delivery error: %v", err)
			return // will be retried by NATS with backoff
		}

		m.Ack()
	}, nats.Durable(util.DurableConsumer),
		nats.ManualAck(),
		nats.Bind(util.StreamName, util.DurableConsumer))

	if err != nil {
		log.Fatalf("subscription failed: %v", err)
	}

	log.Println("worker ready")
	select {}
}
