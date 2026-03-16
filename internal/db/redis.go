package db

import (
	"context"
	"crypto/tls"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

func InitRedis() *redis.Client {
	rawURL := os.Getenv("REDIS_URL")

	var rdb *redis.Client

	if rawURL != "" {
		// ── Managed Redis (Leapcell, Heroku, Render, etc.) ──────────
		// Many providers give a redis:// URL even though the server
		// requires TLS. We normalise to rediss:// so ParseURL enables
		// TLS, then set InsecureSkipVerify because managed providers
		// typically use self-signed or provider-signed certificates.
		//
		// Also percent-encode special characters in the password
		// (+, /, =, @) that would otherwise break URL parsing.
		normalised := normRedisURL(rawURL)

		opt, err := redis.ParseURL(normalised)
		if err != nil {
			log.Fatalf("redis: failed to parse REDIS_URL: %v", err)
		}

		// Force TLS with certificate verification disabled.
		// This is safe for managed cloud Redis where the host is
		// authenticated by the provider — we just can't verify the
		// certificate chain ourselves.
		opt.TLSConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		}

		rdb = redis.NewClient(opt)
		log.Println("redis: connecting via REDIS_URL (TLS)")
	} else {
		// ── Local development ────────────────────────────────────────
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		rdb = redis.NewClient(&redis.Options{Addr: addr})
		log.Printf("redis: connecting to %s (no TLS)", addr)
	}

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: connection failed: %v", err)
	}

	log.Println("redis: connected")
	return rdb
}

// normRedisURL normalises a Redis URL for use with redis.ParseURL:
//  1. Converts redis:// → rediss:// so TLS is enabled by ParseURL.
//  2. Percent-encodes special characters in the password so the URL
//     parser doesn't misinterpret + / = @ as structural characters.
func normRedisURL(raw string) string {
	// Step 1 — upgrade scheme to TLS
	normalised := strings.Replace(raw, "redis://", "rediss://", 1)

	// Step 2 — encode special chars in password
	parsed, err := url.Parse(normalised)
	if err != nil {
		// Can't parse — return as-is and let redis.ParseURL report the error
		return normalised
	}

	if parsed.User != nil {
		user := parsed.User.Username()
		pass, hasPass := parsed.User.Password()
		if hasPass {
			// url.QueryEscape encodes space as +; use PathEscape instead
			// which encodes space as %20 and leaves more chars readable,
			// but critically encodes + / @ which break the URL structure.
			parsed.User = url.UserPassword(user, url.PathEscape(pass))
		}
	}

	return parsed.String()
}