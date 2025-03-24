package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/memory"
	"github.com/redis/go-redis/v9"
)

type CacheJSONWriter[T any] struct {
	cli *redis.Client
}

func NewCacheJsonWriter[T any](memoryCfg config.Memory) memory.CacheWriter[T] {
	rdb := redis.NewClient(GetRedisOptions(memoryCfg))
	return &CacheJSONWriter[T]{cli: rdb}
}

func (c *CacheJSONWriter[T]) Set(key string, data *T, exp time.Duration) error {
	if data == nil {
		return errors.New("cache data should not be null for key: " + key)
	}

	m, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = c.cli.Set(context.Background(), key, m, exp).Err()
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheJSONWriter[T]) Rm(key string) error {
	c.cli.Del(context.Background(), key)
	return nil
}

func (c *CacheJSONWriter[T]) Get(key string) (*T, error) {
	val, _ := c.cli.Get(context.Background(), key).Result()
	if val == "" {
		return nil, nil
	}

	var v T
	err := json.Unmarshal([]byte(val), &v)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func GetRedisOptions(memoryCfg config.Memory) *redis.Options {
	if memoryCfg.Redis.Tls == true {
		return &redis.Options{
			Addr:     memoryCfg.Redis.Url + ":" + memoryCfg.Redis.Port,
			Username: memoryCfg.Redis.User,
			Password: memoryCfg.Redis.Password,
			DB:       0,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}
	} else {
		return &redis.Options{
			Addr:     memoryCfg.Redis.Url + ":" + memoryCfg.Redis.Port,
			Username: memoryCfg.Redis.User,
			Password: memoryCfg.Redis.Password,
			DB:       0,
		}
	}
}
