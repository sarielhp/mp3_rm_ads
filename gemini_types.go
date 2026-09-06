package main

const geminiAdRemovalPrompt = `You are an expert audio editor analyzing a podcast episode.
Your task is to:
1. Identify all non-content intervals to be removed:
   - "advertisement": commercial breaks, sponsor plugs, promotional host-reads (including Hebrew: חסויות, קודי קופון, שיתופי פעולה).
   - "music_interlude": extended transition or filler music without speech longer than 5 seconds.
   - "intro_outro": pre-roll or post-roll theme songs and disclaimers.
2. Provide a verbatim timestamped transcript for the remaining spoken content.

Return ONLY a valid JSON object strictly matching this schema:
{
  "cuts": [
    {"start": 12.5, "end": 45.0, "type": "advertisement", "reason": "Sponsor plug for Wolt"}
  ],
  "segments": [
    {"start": 45.0, "end": 52.3, "text": "ברוכים הבאים לפרק..."}
  ]
}`

type geminiCutItem struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Type   string  `json:"type"`
	Reason string  `json:"reason"`
}

type geminiSegmentItem struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type geminiResponsePayload struct {
	Cuts     []geminiCutItem     `json:"cuts"`
	Segments []geminiSegmentItem `json:"segments"`
}

type geminiChunkInfo struct {
	index    int
	startSec float64
	durSec   float64
	filePath string
}

type geminiChunkResult struct {
	index    int
	startSec float64
	payload  *geminiResponsePayload
}
