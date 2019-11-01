package main

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/artemis-org/sharder/config"
	"github.com/artemis-org/sharder/gateway"
	"github.com/artemis-org/sharder/redis"
)

func main() {
	config.LoadConfig()
	gateway.StartLoginQueue()

	uri := redis.CreateRedisURI(config.Conf.RedisUri)
	client := gateway.CreateRedisClient(uri)

	var wg sync.WaitGroup
	wg.Add(config.Conf.ShardLast - config.Conf.ShardFirst + 1)

	for i := config.Conf.ShardFirst; i <= config.Conf.ShardLast; i++ {
		sessId := ""
		shardCount, err := client.GetShardCount()
		if err != nil && shardCount == strconv.Itoa(config.Conf.ShardTotal) {
			sessId, err = client.GetSessionId(i)
		}

		shard := gateway.NewShard(i, &client, sessId)

		start(shard)
	}

	client.SetShardCount()

	wg.Wait()
}

func start(shard gateway.Shard) {
	err := shard.Connect()
	if err != nil {
		fmt.Println(err.Error())
		start(shard)
	}
}
