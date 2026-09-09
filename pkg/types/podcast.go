package types

import "time"

type PodcastFrequencyInfo struct {
	Type                string    `json:"type"`
	EpisodesAnalyzed    int       `json:"episodes_analyzed"`
	AvgDaysInterval     float64   `json:"avg_days_interval"`
	MedianHoursInterval float64   `json:"median_hours_interval"`
	EpisodesPerWeek     float64   `json:"episodes_per_week"`
	AnalyzedAt          time.Time `json:"analyzed_at"`
}
