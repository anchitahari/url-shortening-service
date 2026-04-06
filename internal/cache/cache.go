package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

func New(addr string) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &Cache{client: client}
}

func (c *Cache) Set(ctx context.Context, code, url string) error {
	return c.client.Set(ctx, code, url, 24*time.Hour).Err()
}

func (c *Cache) Get(ctx context.Context, code string) (string, error) {
	return c.client.Get(ctx, code).Result()
}

func (c *Cache) Delete(ctx context.Context, code string) error {
	return c.client.Del(ctx, code).Err()
}