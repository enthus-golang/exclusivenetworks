package exclusivenetworks

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{"happy_path", `"2026-04-30"`, time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC), false},
		{"empty_string_is_zero", `""`, time.Time{}, false},
		{"null_is_zero", `null`, time.Time{}, false},
		{"malformed_errors", `"not-a-date"`, time.Time{}, true},
		{"rfc3339_not_accepted", `"2026-04-30T00:00:00Z"`, time.Time{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d Date
			err := json.Unmarshal([]byte(tc.input), &d)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.True(t, tc.want.Equal(d.Time), "got %v want %v", d.Time, tc.want)
		})
	}
}

func TestDateMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   Date
		want string
	}{
		{"happy_path", Date{Time: time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)}, `"2026-04-30"`},
		{"zero_emits_empty_string", Date{}, `""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := json.Marshal(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(out))
		})
	}
}

// TestDateRoundTrip protects against the time.Time embedding promoting
// RFC3339 JSON methods that would shadow ours. If MarshalJSON/UnmarshalJSON
// ever stop being defined on Date, this round-trip breaks.
func TestDateRoundTrip(t *testing.T) {
	original := Date{Time: time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)}
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, `"2026-04-30"`, string(encoded))

	var decoded Date
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.True(t, original.Time.Equal(decoded.Time))
}
