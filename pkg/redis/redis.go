package redispkg

import (
	"context"
	"github.com/go-redis/redis/v8"
	"time"
)

var (
	RedisHost     string
	RedisPassword string
)

// SetupRedisClient создает и настраивает клиент Redis для подключения к серверу Redis.
func SetupRedisClient() *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:            RedisHost,
		Password:        RedisPassword,
		DB:              0,
		MaxRetries:      3,
		MinRetryBackoff: 500 * time.Millisecond,
		MaxRetryBackoff: 3 * time.Second,
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			_, err := cn.Ping(ctx).Result()
			return err
		},
	})

	return redisClient
}
