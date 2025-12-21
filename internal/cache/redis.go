package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// Init initializes the Redis connection
func Init() error {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "redis.default.svc.cluster.local"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	client = redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}

// GetClient returns the Redis client
func GetClient() *redis.Client {
	return client
}

// hashCredentials creates a hash of email+password for cache key
func hashCredentials(email, password string) string {
	h := sha256.New()
	h.Write([]byte(email + ":" + password))
	return "auth:" + hex.EncodeToString(h.Sum(nil))[:32]
}

// GetCachedAuth checks if credentials are cached and valid
func GetCachedAuth(ctx context.Context, email, password string) (int64, bool) {
	if client == nil {
		return 0, false
	}
	key := hashCredentials(email, password)
	userID, err := client.Get(ctx, key).Int64()
	if err != nil {
		return 0, false
	}
	return userID, true
}

// CacheAuth caches valid credentials for 15 minutes
func CacheAuth(ctx context.Context, email, password string, userID int64) {
	if client == nil {
		return
	}
	key := hashCredentials(email, password)
	client.Set(ctx, key, userID, 15*time.Minute)
}

// InvalidateAuth removes cached auth for a user (on password change/logout)
func InvalidateAuth(ctx context.Context, email, password string) {
	if client == nil {
		return
	}
	key := hashCredentials(email, password)
	client.Del(ctx, key)
}
