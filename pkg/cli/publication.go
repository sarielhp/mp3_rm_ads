package cli

import (
	"time"
)

func publicationDateTime(date time.Time) string {
	if date.IsZero() {
		return "—"
	}
	return queuePublicationTime(date, time.Now())
}
