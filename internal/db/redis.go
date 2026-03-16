package db

import (
	"context"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

func InitRedis() *redis.Client {
	var rdb *redis.Client

	// Leapcell (and most managed Redis providers) expose a full URL.
	// Fall back to a plain host:port address for local dev.
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Fatalf("redis: invalid REDIS_URL: %v", err)
		}
		rdb = redis.NewClient(opt)
		log.Println("redis: connecting via REDIS_URL")
	} else {
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		rdb = redis.NewClient(&redis.Options{Addr: addr})
		log.Printf("redis: connecting to %s", addr)
	}

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: connection failed: %v", err)
	}

	log.Println("redis: connected")
	return rdb
}