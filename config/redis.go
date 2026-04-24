package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

var Redis *redis.Client
var RedisCTX = context.Background()

// ConnectRedis initializes the Redis client.
func ConnectRedis() {
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")
	dbIndex := 0

	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if val, err := strconv.Atoi(dbStr); err == nil {
			dbIndex = val
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Password:     password,
		DB:           dbIndex,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(RedisCTX, 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Printf("Connected to Redis at %s:%s (DB: %d)", host, port, dbIndex)
	Redis = client
}
