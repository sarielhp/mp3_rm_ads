# Plan: Integrating Gemini 1.5 Flash Audio Processing into `abs`

This document details the architecture, setup, and Go code to integrate Google Cloud Vertex AI's **Gemini 1.5 Flash** into the [`abs`](file:///home/sariel/prog/26/podcasts/abs) podcast processing pipeline.

---

## 1. Executive Summary & Value

### The Shift from Whisper + LLM to Multimodal Audio
* **Previous Architecture**:
  1. Wake up / start remote Compute Engine VM (`cloud8`).
  2. Transcribe full audio with Faster-Whisper on CPU/GPU into timestamped text (~10–15 mins).
  3. Send full transcript text to OpenRouter / LLM to guess ad intervals (~15–20s).
  4. Run `ffmpeg` to cut intervals.
* **New Architecture (Gemini 1.5 Flash on Vertex AI)**:
  1. Stage audio in a temporary GCS bucket (or send inline if < 20MB).
  2. Call Gemini 1.5 Flash in a single multimodal pass (~10–60s).
  3. Gemini listens to the audio waveforms and reads the text simultaneously, detecting:
     - Spoken content transcripts with timestamps.
     - Advertisement and sponsor segments (including Hebrew sponsor cues and tone shifts).
     - Musical interludes and non-speech filler.
  4. Run `ffmpeg` to cut the detected intervals.

### Key Benefits
* **Uses GCP Promotional Credits**: Draws directly from project `vm-on-cloud-sariel` credits via Vertex AI.
* **Zero VM Management**: No waking, sleeping, quotas, or standing disk/NAT fees.
* **Hebrew & Music Detection**: Accurately recognizes Israeli podcast sponsor patterns (*"הפרק בחסות..."*, *"קוד קופון..."*, *Wolt, Fiverr, Monday.com*) and non-speech musical interludes.
* **Cost**: ~$0.07 per hour of audio processed.

---

## 2. Google Cloud Setup & Prerequisites

### 2.1 Enable Required APIs
Ensure Vertex AI and Cloud Storage APIs are active in project `vm-on-cloud-sariel`:
```bash
gcloud services enable aiplatform.googleapis.com storage.googleapis.com --project=vm-on-cloud-sariel
```

### 2.2 Create Audio Staging Bucket
Create a dedicated regional bucket for staging podcast audio files with a 1-day lifecycle rule:
```bash
gcloud storage buckets create gs://abs-audio-staging-sariel --location=us-central1 --project=vm-on-cloud-sariel
```

### 2.3 Authentication via Application Default Credentials (ADC)
Ensure local ADC is logged into your Google account:
```bash
gcloud auth application-default login
```
The Google Cloud Go SDK reads credentials automatically from `~/.config/gcloud/application_default_credentials.json`.

---

## 3. Go Dependencies

Add the official Google Cloud Vertex AI and Cloud Storage SDKs to [`go.mod`](file:///home/sariel/prog/26/podcasts/abs/go.mod):
```bash
go get cloud.google.com/go/vertexai/genai
go get cloud.google.com/go/storage
```

---

## 4. Implementation Design for `abs`

All Go functions must adhere to the **≤ 80 lines per function** limit and follow modular decomposition.

### 4.1 Data Structures & Prompts (`gemini_types.go`)

```go
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
```

---

### 4.2 GCS Audio Staging Helper (`gemini_storage.go`)

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
)

// uploadAudioToGCS uploads a local audio file to the staging bucket.
func uploadAudioToGCS(ctx context.Context, bucketName, localAudioPath string) (string, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	file, err := os.Open(localAudioPath)
	if err != nil {
		return "", fmt.Errorf("failed to open local audio file: %w", err)
	}
	defer file.Close()

	objectName := fmt.Sprintf("audio-staging/%d-%s", time.Now().UnixNano(), filepath.Base(localAudioPath))
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = "audio/mpeg"

	if _, err := io.Copy(wc, file); err != nil {
		_ = wc.Close()
		return "", fmt.Errorf("failed to upload audio to GCS: %w", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS object: %w", err)
	}

	return fmt.Sprintf("gs://%s/%s", bucketName, objectName), nil
}

// deleteGCSObject cleans up the staged audio file after processing.
func deleteGCSObject(ctx context.Context, bucketName, gcsURI string) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return
	}
	defer client.Close()

	prefix := fmt.Sprintf("gs://%s/", bucketName)
	if len(gcsURI) > len(prefix) {
		objName := gcsURI[len(prefix):]
		_ = client.Bucket(bucketName).Object(objName).Delete(ctx)
	}
}
```

---

### 4.3 Vertex AI Gemini Client (`gemini_client.go`)

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/vertexai/genai"
)

// callGeminiAudioProcessor sends the audio GCS URI to Gemini 1.5 Flash.
func callGeminiAudioProcessor(ctx context.Context, projectID, location, gcsURI string) (*geminiResponsePayload, error) {
	client, err := genai.NewClient(ctx, projectID, location)
	if err != nil {
		return nil, fmt.Errorf("failed to create vertex ai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-flash")
	model.ResponseMIMEType = "application/json"
	model.SetTemperature(0.1)

	audioPart := genai.FileData{
		MIMEType: "audio/mpeg",
		FileURI:  gcsURI,
	}

	resp, err := model.GenerateContent(ctx, audioPart, genai.Text(geminiAdRemovalPrompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate content failed: %w", err)
	}

	return parseGeminiContentResponse(resp)
}

// parseGeminiContentResponse extracts and unmarshals the JSON payload from the model candidates.
func parseGeminiContentResponse(resp *genai.GenerateContentResponse) (*geminiResponsePayload, error) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("empty response received from Gemini")
	}

	var jsonText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			jsonText += string(textPart)
		}
	}

	var payload geminiResponsePayload
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini JSON: %w (raw response: %s)", err, jsonText)
	}
	return &payload, nil
}
```

---

### 4.4 Adapter to Native `abs` Data Structures (`gemini_pipeline.go`)

```go
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// convertGeminiToAbsTypes maps Gemini output directly to native abs types.
func convertGeminiToAbsTypes(payload *geminiResponsePayload) (*TranscriptionData, []AdSegment) {
	td := &TranscriptionData{}
	for _, s := range payload.Segments {
		td.Segments = append(td.Segments, TranscriptionSegment{
			Start: s.Start,
			End:   s.End,
			Text:  s.Text,
		})
		td.Text += s.Text + " "
	}

	var ads []AdSegment
	for _, c := range payload.Cuts {
		ads = append(ads, AdSegment{
			Start:  c.Start,
			End:    c.End,
			Reason: fmt.Sprintf("[%s] %s", c.Type, c.Reason),
		})
	}

	return td, ads
}

// ProcessWithGeminiFlash runs the end-to-end Gemini audio workflow.
func ProcessWithGeminiFlash(ctx context.Context, audioPath, projectID, bucketName string) (*TranscriptionData, []AdSegment, error) {
	fmt.Printf("Uploading '%s' to GCS staging bucket...\n", filepath.Base(audioPath))
	gcsURI, err := uploadAudioToGCS(ctx, bucketName, audioPath)
	if err != nil {
		return nil, nil, err
	}
	defer deleteGCSObject(ctx, bucketName, gcsURI)

	fmt.Println("Analyzing audio with Gemini 1.5 Flash (transcription + ad/music detection)...")
	t0 := time.Now()
	payload, err := callGeminiAudioProcessor(ctx, projectID, "us-central1", gcsURI)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("Gemini processing finished in %s!\n", formatClock(time.Since(t0).Seconds()))

	td, ads := convertGeminiToAbsTypes(payload)
	return td, ads, nil
}
```

---

## 5. Integrating with Existing `abs` Pipeline

In [`pipeline.go`](file:///home/sariel/prog/26/podcasts/abs/pipeline.go):
1. **Add engine option**: Support `--whisper-engine=gemini` (or set `gemini` in config).
2. **Unified Execution**:
   ```go
   if wp.Engine == "gemini" {
       td, ads, err := ProcessWithGeminiFlash(ctx, sourceAudioFile, "vm-on-cloud-sariel", "abs-audio-staging-sariel")
       if err != nil {
           return err
       }
       // Save transcript JSON
       saveTranscriptJSON(jsonFile, td)
       // Save cuts JSON directly without calling separate ads.go LLM
       cutsResult := saveCutsJSON(mainMP3File, totalDuration, ads, &selectedProfile, cli.Quiet)
       // Cut audio using existing ffmpeg routine
       executeRecutAudio(sourceAudioFile, precutFile, outputFile, tempOutputFile, mainMP3File, workDir, cutsResult.KeepSegments, totalDuration, config, cli, fileStartTime)
       return nil
   }
   ```

---

## 6. Verification & Testing Checklist

- [ ] Ensure `gcloud auth application-default print-access-token` succeeds.
- [ ] Create staging bucket `gs://abs-audio-staging-sariel`.
- [ ] Run a test episode: `abs proc --whisper-engine=gemini <episode.mp3>`.
- [ ] Confirm output `.cuts.json` accurately captured Hebrew sponsor plugs and transition music.
- [ ] Confirm the temporary GCS audio file was automatically deleted after inference.
- [ ] Verify GCP billing report confirms charges were deducted from promotional credits.
