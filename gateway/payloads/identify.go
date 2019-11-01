package payloads

import "github.com/artemis-org/sharder/config"

type(
	Identify struct {
		Opcode int `json:"op"`
		Data IdentifyData `json:"d"`
	}

	IdentifyData struct {
		Token string `json:"token"`
		Properties Properties `json:"properties"`
		Compress bool `json:"compress"`
		LargeThreshold int `json:"large_threshold"`
		Shard []int `json:"shard"`
		Presence Presence `json:"presence"`
	}

	Properties struct {
		Os string `json:"$os"`
		Browser string `json:"$browser"`
		Device string `json:"$device"`
	}

	Presence struct {
		Game Game `json:"game"`
		Status string `json:"status"`
		Since *int `json:"since"`
		Afk bool `json:"afk"`
	}

	Game struct {
		Name string `json:"name"`
		Type int `json:"type"`
	}
)

func NewIdentify(shardId int) Identify {
	payload := Identify{
		Opcode: 2,
		Data: IdentifyData{
			Token: config.Conf.BotToken,
			Properties: Properties{
				Os: "linux",
				Browser: "vexera",
				Device: "vexera",
			},
			Compress: false, // TODO: Use compression
			LargeThreshold: 250,
			Shard: []int{shardId, config.Conf.ShardTotal},
		},
	}

	if &config.Conf.BotPresence != nil {
		payload.Data.Presence = Presence{
			Game: Game{
				Name: config.Conf.BotPresence,
				Type: 0,
			},
			Status: "online",
			Since: nil,
			Afk: false,
		}
	}

	return payload
}
