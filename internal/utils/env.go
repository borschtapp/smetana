package utils

import (
	"os"
	"strconv"
	"time"
)

func Getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func GetenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value != "" {
		if val, err := strconv.Atoi(value); err == nil {
			return val
		}
	}
	return fallback
}

func GetenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value != "" {
		if val, err := strconv.ParseBool(value); err == nil {
			return val
		}
	}
	return fallback
}

func GetenvDuration(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return d
}
