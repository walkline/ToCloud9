package events

import (
	"encoding/json"
)

type EventToSendGenericPayload struct {
	Version   string      `json:"v"`
	EventType int         `json:"t"`
	Payload   interface{} `json:"p"`
}

type EventToReadGenericPayload struct {
	Version   string          `json:"v"`
	EventType int             `json:"t"`
	Payload   json.RawMessage `json:"p"`
}

func Unmarshal(data []byte, payload interface{}) (int, error) {
	p := EventToReadGenericPayload{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		return 0, err
	}

	err = json.Unmarshal(p.Payload, payload)
	if err != nil {
		return 0, err
	}

	return p.EventType, nil
}
