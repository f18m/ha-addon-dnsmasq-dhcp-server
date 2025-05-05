package uibackend

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	testCases := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"1h", time.Hour, false},
		{"1.5h", 90 * time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"1M", 30 * 24 * time.Hour, false}, // Assuming 30 days in a month
		{"1y", 365 * 24 * time.Hour, false},
		{"-1h", -time.Hour, false},
		{"-1.5h", -90 * time.Minute, false},
		{"-2w", -14 * 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"1.5h30m", 120 * time.Minute, false},                 // mixed units
		{"2.5d", 60 * time.Hour, false},                       // decimal days
		{"1.5h30m2s", 120*time.Minute + 2*time.Second, false}, // more complex cases
		{"2D", 48 * time.Hour, false},
		{"1W", 7 * 24 * time.Hour, false},
		{"1Y", 365 * 24 * time.Hour, false},
		{"-2D", -48 * time.Hour, false},
		{"-1W", -7 * 24 * time.Hour, false},

		// error cases
		{"", 0, true},         // empty string
		{"invalid", 0, true},  // invalid input
		{"1.5h 30m", 0, true}, // space between values
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseDuration(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
				return
			}
			if got != tc.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}
