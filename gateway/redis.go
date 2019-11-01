package gateway

import (
	"fmt"
	"github.com/artemis-org/sharder/config"
	redis2 "github.com/artemis-org/sharder/redis"
	"github.com/go-redis/redis"
	"log"
)

type(
	RedisClient struct {
		redis.Client
	}

	Callback func(string)
)

func CreateRedisClient(uri redis2.RedisURI) RedisClient {
	var threads int
	if &config.Conf.RedisThreads == nil {
		threads = config.Conf.ShardLast - config.Conf.ShardFirst + 11 // One for each listen + 10 extra
	} else {
		threads = config.Conf.RedisThreads
	}

	client := redis.NewClient(&redis.Options{
		Addr: uri.Addr,
		Password: uri.Password,
		DB: 0,
		PoolSize: threads,
	})

	return RedisClient{
		*client,
	}
}

func (client *RedisClient) QueueEvent(raw string) {
	client.RPush("sharder:from", raw)
	fmt.Println(raw)
}

func (client *RedisClient) UpdateCache(raw string) {
	client.RPush("cache:update", raw)
}

func (s *Shard) Listen() {
	key := fmt.Sprintf("sharder:to:%d", s.Id)

	client := s.RedisClient

	go func() {
		for {
			res, err := client.BLPop(0, key).Result()
			fmt.Println(res)
			if err != nil {
				log.Println(err.Error())
			}

			event := res[1]
			s.EventChan <- []byte(event)
		}
	}()
}

func (client *RedisClient) SetSessionId(shardId int, sessionId string) {
	key := fmt.Sprintf("sessionid:%d", shardId)
	client.Set(key, sessionId, 0)
}

func (client *RedisClient) GetSessionId(shardId int) (string, error) {
	key := fmt.Sprintf("sessionid:%d", shardId)
	return client.Get(key).Result()
}

func (client *RedisClient) SetShardCount() {
	client.Set("shardcount", config.Conf.ShardTotal, 0)
}

func (client *RedisClient) GetShardCount() (string, error) {
	return client.Get("shardcount").Result()
}
