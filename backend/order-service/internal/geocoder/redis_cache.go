package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisGeocodeCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisGeocodeCache(client *redis.Client) *RedisGeocodeCache {
	return &RedisGeocodeCache{
		client: client,
		ttl:    30 * 24 * time.Hour, // Кэшируем координаты на 30 дней
	}
}

func (c *RedisGeocodeCache) Get(ctx context.Context, key string) (*Coordinates, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Ключ не найден в кэше
		}
		return nil, fmt.Errorf("redis get error: %w", err)
	}
	var coords Coordinates
	if err := json.Unmarshal([]byte(val), &coords); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached coordinates: %w", err)
	}
	return &coords, nil
}

func (c *RedisGeocodeCache) Set(ctx context.Context, key string, coords *Coordinates) error {
	data, err := json.Marshal(coords)
	if err != nil {
		return fmt.Errorf("failed to marshal coordinates for caching: %w", err)
	}
	err = c.client.Set(ctx, key, data, c.ttl).Err()
	if err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}
	return nil
}
