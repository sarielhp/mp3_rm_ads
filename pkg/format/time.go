package format

import (
	"fmt"
	"math"
)

func FormatTime(seconds float64) string {
	sec := seconds
	hrs := int(sec / 3600)
	mins := int(math.Mod(sec, 3600) / 60)
	secs := math.Mod(sec, 60)
	if hrs > 0 {
		return fmt.Sprintf("%02d:%02d:%04.1f", hrs, mins, secs)
	}
	return fmt.Sprintf("%02d:%04.1f", mins, secs)
}

func FormatClock(seconds float64) string {
	sec := math.Max(math.Round(seconds), 0)
	hrs := int(sec / 3600)
	mins := int(math.Mod(sec, 3600) / 60)
	secs := int(math.Mod(sec, 60))
	if hrs > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hrs, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func FormatSRTTime(seconds float64) string {
	sec := seconds
	hrs := int(sec / 3600)
	mins := int(math.Mod(sec, 3600) / 60)
	secs := int(math.Mod(sec, 60))
	millis := int(math.Mod(sec, 1) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hrs, mins, secs, millis)
}

// FormatMinutes renders an audio length in whole minutes, the unit episode
// lengths are reported in: a 1:40:00 episode reads as "100 minutes".
func FormatMinutes(seconds float64) string {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return "unknown"
	}
	mins := int(math.Round(seconds / 60))
	switch {
	case mins < 1:
		return "<1 minute"
	case mins == 1:
		return "1 minute"
	default:
		return fmt.Sprintf("%d minutes", mins)
	}
}
