package stream

import "time"

type Event struct {
	Event   string
	EventID string
	Time    time.Time
	Payload map[string]interface{}
}
