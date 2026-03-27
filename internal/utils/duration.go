package utils

import (
	"fmt"
	"time"
)

// ResolveInterval parses s as a time.Duration.
// Returns an error if s is empty or not a valid duration.
func ResolveInterval(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}
