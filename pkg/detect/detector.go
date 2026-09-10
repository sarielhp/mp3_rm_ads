package detect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sariel/abs/pkg/types"
)

const SystemPrompt = `You are an expert podcast editor assistant.
Your job is to analyze the timestamped transcript of a podcast episode and identify all advertisement segments, host-read sponsor plugs, promotional breaks, midroll/preroll ads, and sponsor call-outs.

Return ONLY a raw JSON array of objects with the exact start and end seconds of each ad segment, like this:
[
  {"start": 15.0, "end": 65.5, "reason": "Host read sponsor plug for VPN"},
  {"start": 1200.0, "end": 1290.0, "reason": "Midroll ad break"}
]

If NO ads or sponsor plugs are found, return an empty JSON array: []
Do not include markdown formatting or commentary outside the JSON array.`

const KeywordExtractionPrompt = `You are a transcription assistant. Your job is to extract key topics, names, technical terms,
brand names, and unusual words from a podcast transcript segment.

Return ONLY a comma-separated list of 10-20 keywords/phrases (each 1-3 words).
Focus on: guest names, topic-specific jargon, product names, locations, and any words
that are unusual or easily misheard.

Keep each keyword short. Do not include markdown or commentary.`

type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMResponse struct {
	Choices []LLMChoice `json:"choices"`
}

type LLMChoice struct {
	Message LLMMessage `json:"message"`
}

var sharedLLMClient = &http.Client{
	Timeout: 120 * time.Second,
}

var DefaultLLMTimeout = 120 * time.Second

func CallLLMChat(profile types.LLMProfile, sysPrompt, userPrompt string, maxTokens int, timeout time.Duration, apiKey string) (string, error) {
	payload := LLMRequest{
		Model: profile.Model,
		Messages: []LLMMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
		MaxTokens:   maxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	client := sharedLLMClient
	if timeout > 0 && timeout != sharedLLMClient.Timeout {
		client = &http.Client{Timeout: timeout}
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
		}
		req, err := http.NewRequest("POST", profile.URL, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server returned status code %d: %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("server returned status code %d: %s", resp.StatusCode, string(respBody))
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read response: %w", err)
		}

		var llmResp LLMResponse
		if err := json.Unmarshal(respBody, &llmResp); err != nil {
			return "", fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if len(llmResp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}

		return llmResp.Choices[0].Message.Content, nil
	}
	return "", lastErr
}

func DetectAdsLLM(transcriptText string, profile types.LLMProfile, apiKey string) ([]types.AdSegment, error) {
	return DetectAdsLLMTimeout(transcriptText, profile, apiKey, DefaultLLMTimeout)
}

func DetectAdsLLMTimeout(transcriptText string, profile types.LLMProfile, apiKey string, timeout time.Duration) ([]types.AdSegment, error) {
	if profile.URL == "" {
		return nil, nil
	}
	userPrompt := fmt.Sprintf("Here is the podcast transcript with timestamps in seconds:\n\n%s", transcriptText)
	content, err := CallLLMChat(profile, SystemPrompt, userPrompt, 0, timeout, apiKey)
	if err != nil {
		return nil, fmt.Errorf("LLM ad detection failed: %w", err)
	}
	return ExtractJSONArray(content)
}

func ExtractJSONArray(content string) ([]types.AdSegment, error) {
	start := strings.IndexByte(content, '[')
	if start < 0 {
		return nil, fmt.Errorf("no JSON array start found in response")
	}

	end := -1
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(content); i++ {
		c := content[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i
				i = len(content)
			}
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("no matching JSON array end found in response")
	}

	var ads []types.AdSegment
	if err := json.Unmarshal([]byte(content[start:end+1]), &ads); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ads JSON: %w", err)
	}
	return ads, nil
}

func ExtractKeywordsLLM(transcriptText string, profile types.LLMProfile, apiKey string, quiet bool) string {
	userPrompt := fmt.Sprintf("Extract keywords from this podcast transcript segment:\n\n%s", transcriptText)
	content, err := CallLLMChat(profile, KeywordExtractionPrompt, userPrompt, 200, 60*time.Second, apiKey)
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Error during keyword extraction: %v\n", err)
		}
		return ""
	}

	var keywords []string
	var current strings.Builder
	for _, ch := range content {
		if ch == ',' || ch == '[' || ch == ']' || ch == '"' {
			if current.Len() > 0 {
				keywords = append(keywords, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		keywords = append(keywords, current.String())
	}

	var cleaned []string
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			cleaned = append(cleaned, kw)
		}
	}
	if len(cleaned) > 30 {
		cleaned = cleaned[:30]
	}

	return strings.Join(cleaned, ", ")
}
