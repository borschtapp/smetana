package utils

import (
	"strings"

	"github.com/google/uuid"
)

// ContainsFold checks if the target string is in the list of strings, ignoring case.
func ContainsFold(target string, list ...string) bool {
	for _, s := range list {
		if strings.EqualFold(target, s) {
			return true
		}
	}
	return false
}

// CsvSplit splits a comma-separated string into a slice of strings, trimming whitespace and converting to lowercase.
func CsvSplit(target string) []string {
	var result []string
	if len(target) != 0 {
		for _, raw := range strings.Split(target, ",") {
			result = append(result, strings.ToLower(strings.TrimSpace(raw)))
		}
	}
	return result
}

// CsvSplitUUID splits a comma-separated string into a slice of UUIDs, ignoring invalid UUIDs.
func CsvSplitUUID(target string) []uuid.UUID {
	var result []uuid.UUID
	if len(target) != 0 {
		for _, raw := range strings.Split(target, ",") {
			if id, err := uuid.Parse(strings.TrimSpace(raw)); err == nil {
				result = append(result, id)
			}
		}
	}
	return result
}

func EnsureSuffix(s, suffix string) string {
	if strings.HasSuffix(s, suffix) {
		return s
	}
	return s + suffix
}
