package db

import (
	"context"
	"crypto/tls"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// InitRedis connects to Redis using explicit environment variables.
//
// For production (Leapcell), set three env vars individually:
//
//	REDIS_HOST     — hostname only, e.g. pulsarmini-redis-xxxx.leapcell.cloud
//	REDIS_PORT     — port only, e.g. 6379
//	REDIS_PASSWORD — raw password, no encoding needed
//
// For local development, set nothing — defaults to localhost:6379 with no auth.
//
// Why not REDIS_URL?
// Managed Redis passwords routinely contain +, /, =, @ which are
// structurally significant in URLs. Every URL parser corrupts them.
// Discrete variables avoid the problem entirely.
func InitRedis() *redis.Client {
	host     := os.Getenv("REDIS_HOST")
	port     := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")

	var rdb *redis.Client

	if host != "" {
		// ── Production — managed / serverless Redis ──────────────
		if port == "" {
			port = "6379"
		}
		addr := host + ":" + port

		rdb = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,

			// TLS is required by Leapcell Redis.
			// InsecureSkipVerify is necessary because managed providers
			// use self-signed or provider-signed certificates that Go
			// cannot verify without the provider's CA bundle.
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},

			// ── Pool tuning for serverless Redis ─────────────────
			// Leapcell Redis is serverless — it suspends when idle.
			// Default pool settings keep idle connections open for
			// minutes, which then time out with i/o timeout errors.
			// These settings ensure the pool never holds stale
			// connections and dials fresh ones when needed.

			// Maximum open connections in the pool
			PoolSize: 5,

			// How long to wait for a connection from the pool
			PoolTimeout: 10 * time.Second,

			// Close idle connections after 30s — shorter than the
			// serverless suspend window to avoid stale connections
			ConnMaxIdleTime: 30 * time.Second,

			// Maximum lifetime of any connection regardless of use
			ConnMaxLifetime: 5 * time.Minute,

			// Dial timeout — how long to wait when opening a new
			// connection to a cold (suspended) Redis instance
			DialTimeout: 10 * time.Second,

			// Read/write timeout per command
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,

			// Retry failed commands up to 3 times with backoff —
			// handles the brief delay when serverless Redis wakes up
			MaxRetries:      3,
			MinRetryBackoff: 500 * time.Millisecond,
			MaxRetryBackoff: 2 * time.Second,
		})
		log.Printf("redis: connecting to %s (TLS)", addr)
	} else {
		// ── Local development — plain, no auth ───────────────────
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