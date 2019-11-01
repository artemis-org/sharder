package redis

import "net/url"

type(
	RedisURI struct {
		Addr string
		Password string
	}
)

func CreateRedisURI(raw string) RedisURI {
	parsed, err := url.Parse(raw); if err != nil {
		panic(err)
	}

	passwd, _ := parsed.User.Password()

	return RedisURI{
		Addr: parsed.Host,
		Password: passwd,
	}
}
