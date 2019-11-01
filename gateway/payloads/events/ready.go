package events

import "encoding/json"

type(
	Ready struct {
		Opcode int `json:"op"`
		EventName string `json:"t"`
		ReadyData `json:"d"`
	}

	ReadyData struct {
		SessionId string `json:"session_id"`
	}
)

func NewReady(raw []byte) (Ready, error) {
	var payload Ready
	err := json.Unmarshal(raw, &payload); if err != nil {
		return Ready{}, err
	}

	return payload, nil
}
