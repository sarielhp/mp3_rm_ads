package cli

import (
	"abs/pkg/podcast"
	"time"
)

func preparePublicationSource(action string, cfg Config, cli CLIOptions) error {
	podcast.SetPublicationSource("", nil)
	return nil
}

func publicationDateTime(date time.Time) string {
	if date.IsZero() {
		return "—"
	}
	return queuePublicationTime(date, time.Now())
}
