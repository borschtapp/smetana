package utils

import (
	"os"
	"strconv"
)

func Getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func GetenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if len(value) != 0 {
		if val, err := strconv.Atoi(value); err == nil {
			return val
		}
	}
	return fallback
}

func GetenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if len(value) != 0 {
		if val, err := strconv.ParseBool(value); err == nil {
			return val
		}
	}
	return fallback
}
