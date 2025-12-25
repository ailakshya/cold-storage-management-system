package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Room Visualization Cache Keys
const (
	RoomStatsKey    = "room:stats"
	FloorDataKeyFmt = "room:floor:%s:%s"
)

var client *redis.Client

// Init initializes the Redis connection
func Init() error {
	// K8s sets REDIS_SERVICE_HOST and REDIS_SERVICE_PORT for services
	host := os.Getenv("REDIS_SERVICE_HOST")
	if host == "" {
		host = "redis"  // fallback to service name
	}
	port := os.Getenv("REDIS_SERVICE_PORT")
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
		// Close the failed client and set to nil for graceful degradation
		client.Close()
		client = nil
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

// ============================================
// Room Visualization Cache Functions
// ============================================

// GetCachedRoomStats returns cached room stats if available
func GetCachedRoomStats(ctx context.Context) ([]byte, bool) {
	if client == nil {
		return nil, false
	}
	data, err := client.Get(ctx, RoomStatsKey).Bytes()
	if err != nil {
		return nil, false
	}
	return data, true
}

// CacheRoomStats caches room stats for 5 minutes
func CacheRoomStats(ctx context.Context, data []byte) {
	if client == nil {
		return
	}
	client.Set(ctx, RoomStatsKey, data, 5*time.Minute)
}

// GetCachedFloorData returns cached floor gatar data if available
func GetCachedFloorData(ctx context.Context, roomNo, floor string) ([]byte, bool) {
	if client == nil {
		return nil, false
	}
	key := fmt.Sprintf(FloorDataKeyFmt, roomNo, floor)
	data, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return data, true
}

// CacheFloorData caches floor data for 5 minutes
func CacheFloorData(ctx context.Context, roomNo, floor string, data []byte) {
	if client == nil {
		return
	}
	key := fmt.Sprintf(FloorDataKeyFmt, roomNo, floor)
	client.Set(ctx, key, data, 5*time.Minute)
}

// InvalidateRoomCache clears all room visualization cache
func InvalidateRoomCache(ctx context.Context) {
	if client == nil {
		return
	}
	// Delete stats cache
	client.Del(ctx, RoomStatsKey)
	// Delete all floor caches using pattern
	keys, err := client.Keys(ctx, "room:floor:*").Result()
	if err == nil && len(keys) > 0 {
		client.Del(ctx, keys...)
	}
}

// ============================================
// Generic Cache Functions
// ============================================

// GetCached returns cached data for a key
func GetCached(ctx context.Context, key string) ([]byte, bool) {
	if client == nil {
		return nil, false
	}
	data, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return data, true
}

// SetCached stores data with a TTL
func SetCached(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if client == nil {
		return
	}
	client.Set(ctx, key, data, ttl)
}

// ============================================
// Cache Invalidation Functions
// ============================================

// InvalidatePattern removes all keys matching a glob pattern
func InvalidatePattern(ctx context.Context, pattern string) {
	if client == nil {
		return
	}
	keys, err := client.Keys(ctx, pattern).Result()
	if err == nil && len(keys) > 0 {
		client.Del(ctx, keys...)
	}
}

// InvalidateKeys removes specific cache keys
func InvalidateKeys(ctx context.Context, keys ...string) {
	if client == nil || len(keys) == 0 {
		return
	}
	client.Del(ctx, keys...)
}

// ============================================
// Entity-Based Cache Invalidators
// ============================================

// InvalidateCustomerCaches clears all customer-related caches
// Called when: CreateCustomer, UpdateCustomer, DeleteCustomer
func InvalidateCustomerCaches(ctx context.Context) {
	InvalidatePattern(ctx, "customers:*")
	InvalidateKeys(ctx, "account:summary")
}

// InvalidateEntryCaches clears all entry-related caches
// Called when: CreateEntry, UpdateEntry, DeleteEntry
func InvalidateEntryCaches(ctx context.Context) {
	InvalidatePattern(ctx, "entries:*")
	InvalidateKeys(ctx, "account:summary", "entry:room:summary")
	// Also invalidate room cache since entries affect room occupancy
	InvalidateRoomCache(ctx)
}

// InvalidateRoomEntryCaches clears all room entry-related caches
// Called when: CreateRoomEntry, UpdateRoomEntry, DeleteRoomEntry
func InvalidateRoomEntryCaches(ctx context.Context) {
	InvalidatePattern(ctx, "room:*")
	InvalidateKeys(ctx, "account:summary", "entry:room:summary")
}

// InvalidateGatePassCaches clears all gate pass-related caches
// Called when: CreateGatePass, ApproveGatePass, CompleteGatePass, RecordPickup
func InvalidateGatePassCaches(ctx context.Context) {
	InvalidatePattern(ctx, "gate_pass:*")
	InvalidateKeys(ctx, "account:summary")
}

// InvalidateGuardEntryCaches clears all guard entry-related caches
// Called when: CreateGuardEntry, ProcessGuardEntry, DeleteGuardEntry
func InvalidateGuardEntryCaches(ctx context.Context) {
	InvalidatePattern(ctx, "guard:*")
	InvalidateKeys(ctx, "entry:room:summary")
}

// InvalidateUserCaches clears all user-related caches
// Called when: CreateUser, UpdateUser, DeleteUser, ToggleStatus
func InvalidateUserCaches(ctx context.Context) {
	InvalidatePattern(ctx, "users:*")
}

// InvalidateSettingCaches clears all setting-related caches
// Called when: UpdateSetting
func InvalidateSettingCaches(ctx context.Context) {
	InvalidatePattern(ctx, "settings:*")
	// Settings like rent_per_item affect account calculations
	InvalidateKeys(ctx, "account:summary")
}

// InvalidatePaymentCaches clears all payment-related caches
// Called when: CreatePayment
func InvalidatePaymentCaches(ctx context.Context) {
	InvalidatePattern(ctx, "payments:*")
	InvalidateKeys(ctx, "account:summary")
}

// InvalidateAllBusinessCaches clears ALL business data caches
// Called when: ApproveSeasonRequest (archives all data - full reset)
func InvalidateAllBusinessCaches(ctx context.Context) {
	patterns := []string{
		"customers:*", "entries:*", "room:*", "gate_pass:*",
		"guard:*", "payments:*", "account:*", "entry:room:*",
		"settings:*",
	}
	for _, p := range patterns {
		InvalidatePattern(ctx, p)
	}
}
