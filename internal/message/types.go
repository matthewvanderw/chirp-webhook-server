package message

import "encoding/json"

type DestConfig struct {
	URL       string `json:"url"`
	AuthType  string `json:"auth_type"`
	AuthValue string `json:"auth_value"`
}

type WebhookMessage struct {
	Dest   DestConfig      `json:"dest"`
	Body   json.RawMessage `json:"body"`
	Header string          `json:"header"`
}
