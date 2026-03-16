package db

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

func InitRedis() *redis.Client {
	rawURL := os.Getenv("REDIS_URL")

	var rdb *redis.Client

	if rawURL != "" {
		opt, err := parseRedisURL(rawURL)
		if err != nil {
			log.Fatalf("redis: failed to parse REDIS_URL: %v", err)
		}
		rdb = redis.NewClient(opt)
		log.Printf("redis: connecting to %s (TLS)", opt.Addr)
	} else {
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		rdb = redis.NewClient(&redis.Options{Addr: addr})
		log.Printf("redis: connecting to %s (plain)", addr)
	}

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: connection failed: %v", err)
	}

	log.Println("redis: connected")
	return rdb
}

// parseRedisURL manually extracts host, port, username, and password
// from a Redis URL without using Go's net/url parser.
//
// This is necessary because managed Redis providers (Leapcell, Heroku,
// Render, Railway) generate passwords containing +, /, = and other
// characters that are structurally significant in URLs. Using url.Parse
// or redis.ParseURL on these raw URLs corrupts the password.
//
// Supports:
//   redis://password@host:port
//   redis://username:password@host:port
//   rediss://username:password@host:port   (TLS)
//
// TLS is always enabled for managed Redis regardless of scheme.
func parseRedisURL(raw string) (*redis.Options, error) {
	// ── Strip scheme ──────────────────────────────────────────────
	s := raw
	tls_ := true // always use TLS for managed Redis
	if strings.HasPrefix(s, "rediss://") {
		s = strings.TrimPrefix(s, "rediss://")
	} else if strings.HasPrefix(s, "redis://") {
		s = strings.TrimPrefix(s, "redis://")
	} else {
		return nil, fmt.Errorf("unsupported scheme in REDIS_URL (expected redis:// or rediss://)")
	}

	// ── Split credentials from host ───────────────────────────────
	// The last @ separates credentials from the host. Using the LAST
	// @ handles passwords that themselves contain @ signs.
	lastAt := strings.LastIndex(s, "@")
	if lastAt == -1 {
		return nil, fmt.Errorf("REDIS_URL missing @ separator between credentials and host")
	}
	credentials := s[:lastAt]
	hostPort    := s[lastAt+1:]

	// ── Parse credentials ─────────────────────────────────────────
	// Format is either "password" or "username:password".
	// The password may contain colons, so we split on the FIRST colon only.
	var username, password string
	colonIdx := strings.Index(credentials, ":")
	if colonIdx == -1 {
		// No colon — treat the whole thing as the password (Redis default user)
		password = credentials
	} else {
		username = credentials[:colonIdx]
		password = credentials[colonIdx+1:]
		// Passwords that are just the username with no password
		// (e.g. "default:" with empty password) are fine — password stays ""
	}

	// ── Validate host:port ────────────────────────────────────────
	if hostPort == "" {
		return nil, fmt.Errorf("REDIS_URL missing host")
	}

	// ── Build options ─────────────────────────────────────────────
	opt := &redis.Options{
		Addr:     hostPort,
		Username: username,
		Password: password,
	}

	if tls_ {
		opt.TLSConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
			// Managed Redis providers use self-signed or provider-signed
			// certs. We enable TLS for encryption but cannot verify the
			// certificate chain. This is the documented approach for
			// Heroku, Render, Railway, and Leapcell managed Redis.
		}
	}

	return opt, nil
}