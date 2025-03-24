package util

import (
	"os"
	"strconv"
	"time"
)

func SetEnvString(key string, value string) error {
	return os.Setenv(key, value)
}

func UnsetEnv(key string) error {
	return os.Unsetenv(key)
}

// Returns true, if the env var is filled
func CheckEnv(key string) bool {
	return os.Getenv(key) != ""
}

// Env will get os env variable with def as default value
func Env(env, def string) string {
	e := os.Getenv(env)

	if e == "" {
		e = def
	}

	return e
}

// EnvBool converts environment value to boolean
func EnvBool(env string, def bool) bool {
	e, err := strconv.ParseBool(os.Getenv(env))
	if err != nil {
		return def
	}

	return e
}

// EnvInt converts environment value to int
func EnvInt(env string, def int) int {
	return int(EnvInt64(env, int64(def)))
}

// EnvInt64 converts environment value to int64
func EnvInt64(env string, def int64) int64 {
	e, err := strconv.ParseInt(os.Getenv(env), 10, 64)
	if err != nil {
		return def
	}

	return e
}

// EnvTimeDuration converts environment value to int64
func EnvTimeDuration(env string, def time.Duration) time.Duration {
	e, err := strconv.ParseInt(os.Getenv(env), 10, 64)
	if err != nil {
		return def
	}

	return time.Duration(e) * time.Second
}
