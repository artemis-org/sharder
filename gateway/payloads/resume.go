package payloads

import "github.com/artemis-org/sharder/config"

type (
	Resume struct {
		Opcode int        `json:"op"`
		Data   ResumeData `json:"d"`
	}

	ResumeData struct {
		Token          string `json:"token"`
		SessionId      string `json:"session_id"`
		SequenceNumber int    `json:"seq"`
	}
)

func NewResume(sessionId string, sequence int) Resume {
	return Resume{
		Opcode: 6,
		Data: ResumeData{
			Token:          config.Conf.BotToken,
			SessionId:      sessionId,
			SequenceNumber: sequence,
		},
	}
}
