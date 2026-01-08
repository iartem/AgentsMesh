package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis configuration
type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// Cache provides caching functionality using Redis
type Cache struct {
	client *redis.Client
}

// New creates a new Cache instance
func New(cfg *Config) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Cache{client: client}, nil
}

// Close closes the Redis connection
func (c *Cache) Close() error {
	return c.client.Close()
}

// Client returns the underlying Redis client
func (c *Cache) Client() *redis.Client {
	return c.client
}

// Set stores a value with expiration
func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	return c.client.Set(ctx, key, data, expiration).Err()
}

// Get retrieves a value by key
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrNotFound
		}
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete removes a key
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SetNX sets a value only if the key doesn't exist (for distributed locking)
func (c *Cache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}
	return c.client.SetNX(ctx, key, data, expiration).Result()
}

// Increment increments a counter
func (c *Cache) Increment(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// Expire sets expiration on a key
func (c *Cache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// TTL returns the remaining time to live of a key
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// Keys returns keys matching a pattern
func (c *Cache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return c.client.Keys(ctx, pattern).Result()
}

// Hash operations

// HSet sets a field in a hash
func (c *Cache) HSet(ctx context.Context, key, field string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	return c.client.HSet(ctx, key, field, data).Err()
}

// HGet gets a field from a hash
func (c *Cache) HGet(ctx context.Context, key, field string, dest interface{}) error {
	data, err := c.client.HGet(ctx, key, field).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrNotFound
		}
		return err
	}
	return json.Unmarshal(data, dest)
}

// HGetAll gets all fields from a hash
func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

// HDel deletes fields from a hash
func (c *Cache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
}

// List operations

// LPush pushes values to the left of a list
func (c *Cache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return c.client.LPush(ctx, key, values...).Err()
}

// RPush pushes values to the right of a list
func (c *Cache) RPush(ctx context.Context, key string, values ...interface{}) error {
	return c.client.RPush(ctx, key, values...).Err()
}

// LRange gets a range of elements from a list
func (c *Cache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.LRange(ctx, key, start, stop).Result()
}

// LTrim trims a list to the specified range
func (c *Cache) LTrim(ctx context.Context, key string, start, stop int64) error {
	return c.client.LTrim(ctx, key, start, stop).Err()
}

// Pub/Sub operations

// Publish publishes a message to a channel
func (c *Cache) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return c.client.Publish(ctx, channel, data).Err()
}

// Subscribe subscribes to channels
func (c *Cache) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// PSubscribe subscribes to channels matching patterns
func (c *Cache) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	return c.client.PSubscribe(ctx, patterns...)
}

// Errors
var (
	ErrNotFound = fmt.Errorf("key not found")
)

// Key prefixes for different data types
const (
	PrefixSession     = "session:"
	PrefixUser        = "user:"
	PrefixOrg         = "org:"
	PrefixRunner      = "runner:"
	PrefixChannel     = "channel:"
	PrefixRateLimit   = "ratelimit:"
	PrefixLock        = "lock:"
	PrefixPubSub      = "pubsub:"
)

// SessionKey returns a cache key for a session
func SessionKey(sessionKey string) string {
	return PrefixSession + sessionKey
}

// UserKey returns a cache key for a user
func UserKey(userID int64) string {
	return fmt.Sprintf("%s%d", PrefixUser, userID)
}

// OrgKey returns a cache key for an organization
func OrgKey(orgID int64) string {
	return fmt.Sprintf("%s%d", PrefixOrg, orgID)
}

// RunnerKey returns a cache key for a runner
func RunnerKey(runnerID int64) string {
	return fmt.Sprintf("%s%d", PrefixRunner, runnerID)
}

// ChannelKey returns a cache key for a channel
func ChannelKey(channelID int64) string {
	return fmt.Sprintf("%s%d", PrefixChannel, channelID)
}

// RateLimitKey returns a cache key for rate limiting
func RateLimitKey(identifier string) string {
	return PrefixRateLimit + identifier
}

// LockKey returns a cache key for distributed locking
func LockKey(resource string) string {
	return PrefixLock + resource
}

// PubSubChannel returns a pub/sub channel name
func PubSubChannel(channelType string, id int64) string {
	return fmt.Sprintf("%s%s:%d", PrefixPubSub, channelType, id)
}
