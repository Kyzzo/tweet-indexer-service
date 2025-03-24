package memory

import "time"

type CacheWriter[T any] interface {
	Set(key string, data *T, exp time.Duration) error
	Get(key string) (*T, error)
	Rm(key string) error
}
