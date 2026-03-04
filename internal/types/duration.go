package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sosodev/duration"
)

type Duration time.Duration

// MarshalJSON converts the duration to seconds for JSON output
func (d Duration) MarshalJSON() ([]byte, error) {
	if d == 0 {
		return []byte("null"), nil
	}
	seconds := int64(time.Duration(d).Seconds())
	return json.Marshal(seconds)
}

// UnmarshalJSON parses JSON input which can be either:
// - An integer representing seconds (e.g., 1200 for 20 minutes)
// - An ISO 8601 duration string (e.g., "PT20M" for 20 minutes)
func (d *Duration) UnmarshalJSON(b []byte) error {
	var seconds int64
	if err := json.Unmarshal(b, &seconds); err == nil {
		*d = Duration(time.Duration(seconds) * time.Second)
		return nil
	}

	var isoStr string
	if err := json.Unmarshal(b, &isoStr); err != nil {
		return fmt.Errorf("duration must be either an integer (seconds) or ISO 8601 string: %w", err)
	}

	isoDuration, err := duration.Parse(isoStr)
	if err != nil {
		return fmt.Errorf("invalid ISO 8601 duration: %w", err)
	}

	*d = Duration(isoDuration.ToTimeDuration())
	return nil
}

// Value implements the driver.Valuer interface for database storage
// Stores as nanoseconds (int64) for consistency with Go's time.Duration
func (d Duration) Value() (driver.Value, error) {
	return int64(time.Duration(d)), nil
}

// Scan implements the sql.Scanner interface for database retrieval
func (d *Duration) Scan(value interface{}) error {
	if value == nil {
		*d = 0
		return nil
	}

	switch v := value.(type) {
	case int64:
		*d = Duration(time.Duration(v))
		return nil
	case []byte:
		var i int64
		if _, err := fmt.Sscan(string(v), &i); err != nil {
			return err
		}
		*d = Duration(time.Duration(i))
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Duration", value)
	}
}

// ToDuration converts Duration back to time.Duration
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// Seconds returns the duration as a number of seconds
func (d Duration) Seconds() float64 {
	return time.Duration(d).Seconds()
}

// DurationFromISO8601 creates a Duration from an ISO 8601 duration string
func DurationFromISO8601(iso string) (Duration, error) {
	d, err := duration.Parse(iso)
	if err != nil {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %w", err)
	}
	return Duration(d.ToTimeDuration()), nil
}
