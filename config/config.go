package config

import (
	"github.com/magiconair/properties"
)

type(
	Config struct {
		BotToken string `properties:"DISCORD_TOKEN,required"`
		BotPresence string `properties:"DISCORD_PRESENCE"`

		ShardFirst int `properties:"SHARD_ID_FIRST"`
		ShardLast int `properties:"SHARD_ID_LAST"`
		ShardTotal int `properties:"SHARD_COUNT"`

		RedisUri string `properties:"REDIS_URI"`
		RedisThreads int `properties:"REDIS_THREADS"`

		OauthId string `properties:"OAUTH_ID"`
		OauthSecret string `properties:"OAUTH_SECRET"`
		OauthRedirectUri string `properties:"OAUTH_REDIRECT_URI"`
	}
)

var(
	Conf Config
)

func LoadConfig() {
	err := properties.MustLoadFile(".env", properties.UTF8).Decode(&Conf); if err != nil {
		panic(err)
	}
}
