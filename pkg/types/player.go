package types

type PlayerTrack struct {
	Title    string  `json:"title"`
	Podcast  string  `json:"podcast"`
	Path     string  `json:"path"`
	Duration float64 `json:"duration"`
}

type AudioSink struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

type PlayerStatusDTO struct {
	IsRunning bool    `json:"is_running"`
	IsPaused  bool    `json:"is_paused"`
	Title     string  `json:"title"`
	Podcast   string  `json:"podcast,omitempty"`
	Path      string  `json:"path,omitempty"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	Volume    int     `json:"volume"`
}

type PlayerView struct {
	Has            bool          `json:"has"`
	Title          string        `json:"title"`
	Podcast        string        `json:"podcast"`
	Path           string        `json:"path"`
	IsPlaying      bool          `json:"is_playing"`
	IsPaused       bool          `json:"is_paused"`
	Position       float64       `json:"position"`
	Duration       float64       `json:"duration"`
	Volume         int           `json:"volume"`
	Muted          bool          `json:"muted"`
	CurrentSpeaker string        `json:"current_speaker"`
	LastError      string        `json:"last_error"`
	Queue          []PlayerTrack `json:"queue"`
}

type PlayQueuePersist struct {
	Current  *PlayerTrack  `json:"current,omitempty"`
	Queue    []PlayerTrack `json:"queue"`
	Position float64       `json:"position,omitempty"`
}

type UnifiedQueueItem struct {
	Track     PlayerTrack `json:"track"`
	IsCurrent bool        `json:"is_current"`
	IsPlaying bool        `json:"is_playing"`
	IsPaused  bool        `json:"is_paused"`
	Position  float64     `json:"position"`
	Duration  float64     `json:"duration"`
}
