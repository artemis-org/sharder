package events

import "encoding/json"

type Event struct {
	Opcode int `json:"op"`
	SequenceNumber *int `json:"s"`
	EventName string `json:"t"`
}

func NewEvent(raw []byte) (Event, error) {
	var payload Event
	err := json.Unmarshal(raw, &payload); if err != nil {
		return Event{}, err
	}

	return payload, nil
}
