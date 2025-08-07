package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/matthewvanderw/chirp-webhook-server/internal/message"
	"github.com/matthewvanderw/chirp-webhook-server/internal/util"
)

const (
	defaultPort   = "9000"
	destHeader    = "X-Dest"
	expectedParts = 3 // url|auth_type|auth_value
)

func main() {
	nc, js := util.Connect()
	defer nc.Drain()

	// --- Webhook handler ---
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		destHeaders := r.Header[destHeader]
		if len(destHeaders) == 0 {
			log.Printf("no %s headers found", destHeader)
			http.Error(w, "missing X-Dest headers", http.StatusBadRequest)
			return
		}

		for _, encoded := range destHeaders {
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				log.Printf("invalid base64 in %s: %v", destHeader, err)
				continue
			}

			parts := strings.SplitN(string(decoded), "|", expectedParts)
			if len(parts) != expectedParts {
				log.Printf("invalid format in %s: expected url|auth_type|auth_value, got: %q", destHeader, decoded)
				continue
			}

			msg := message.WebhookMessage{
				Dest: message.DestConfig{
					URL:       parts[0],
					AuthType:  parts[1],
					AuthValue: parts[2],
				},
				Body:   json.RawMessage(body),
				Header: destHeader,
			}

			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("failed to marshal message: %v", err)
				continue
			}

			if _, err := js.Publish(util.StreamSubject, data); err != nil {
				log.Printf("failed to publish to NATS: %v", err)
			} else {
				log.Printf("published webhook to %s", msg.Dest.URL)
			}
		}

		w.WriteHeader(http.StatusAccepted)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
