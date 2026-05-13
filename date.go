package exclusivenetworks

import (
	"bytes"
	"fmt"
	"time"
)

// dateLayout matches AccessNow's wire format for date-only fields.
const dateLayout = "2006-01-02"

// Date is a calendar date as it appears on AccessNow quote lines.
// It wraps time.Time and parses AccessNow's "YYYY-MM-DD" wire format.
//
// JSON methods are defined directly on Date because the embedded
// time.Time's MarshalJSON/UnmarshalJSON expect RFC3339 — leaving them
// promoted would silently break round-tripping with AccessNow's date-only
// format.
type Date struct {
	time.Time
}

// UnmarshalJSON parses AccessNow's "YYYY-MM-DD" wire format. Empty string
// and JSON null both decode to a zero Date.
func (d *Date) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) || bytes.Equal(data, []byte(`""`)) {
		d.Time = time.Time{}
		return nil
	}
	s := string(bytes.Trim(data, `"`))
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("exclusivenetworks: parse date %q: %w", s, err)
	}
	d.Time = t
	return nil
}

// MarshalJSON emits AccessNow's "YYYY-MM-DD" wire format. A zero Date
// emits the empty string so it round-trips cleanly with UnmarshalJSON.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + d.Format(dateLayout) + `"`), nil
}
