package utils

import "time"

func Schedule(callback func(args ...interface{}), interval time.Duration, args ...interface{}) {
	for {
		callback(args...)
		time.Sleep(interval)
	}
}
