package db

import (
	"context"
	"crypto/tls"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// InitRedis connects to Redis using explicit environment variables.
//
// For production (Leapcell), set these three env vars individually:
//   REDIS_HOST     — hostname only, e.g. pulsarmini-redis-xxxx.leapcell.cloud
//   REDIS_PORT     — port only, e.g. 6379
//   REDIS_PASSWORD — raw password, no encoding needed
//
// For local development, set nothing — defaults to localhost:6379 with no auth.
//
// Why not use REDIS_URL?
// Managed Redis passwords routinely contain +, /, =, @ which are
// structurally significant in URLs. Every URL parser (including Go's
// net/url and redis.ParseURL) corrupts these passwords during parsing.
// Using discrete variables avoids the problem entirely.
func InitRedis() *redis.Client {
	host     := os.Getenv("REDIS_HOST")
	port     := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")

	var rdb *redis.Client

	if host != "" {
		// Production — managed Redis with TLS
		if port == "" {
			port = "6379"
		}
		addr := host + ":" + port

		rdb = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
				// Managed Redis providers use self-signed or
				// provider-signed certs. TLS is enabled for
				// encryption; certificate chain verification
				// is skipped as it is not possible without
				// the provider's CA bundle.
			},
		})
		log.Printf("redis: connecting to %s (TLS)", addr)
	} else {
		// Local development — plain connection, no auth
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		rdb = redis.NewClient(&redis.Options{
			Addr: addr,
		})
		log.Printf("redis: connecting to %s (plain)", addr)
	}

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: connection failed: %v", err)
	}

	log.Println("redis: connected")
	return rdb
}