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

	client := &http.Client{Timeout: timeout}
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
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("server returned status code %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
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

func DetectAdsLLM(transcriptText string, profile types.LLMProfile, apiKey string) ([]types.AdSegment, error) {
	if profile.URL == "" {
		return nil, nil
	}
	userPrompt := fmt.Sprintf("Here is the podcast transcript with timestamps in seconds:\n\n%s", transcriptText)
	content, err := CallLLMChat(profile, SystemPrompt, userPrompt, 0, 30*time.Second, apiKey)
	if err != nil {
		return nil, fmt.Errorf("LLM ad detection failed: %w", err)
	}
	return ExtractJSONArray(content), nil
}

func ExtractJSONArray(content string) []types.AdSegment {
	start := strings.IndexByte(content, '[')
	if start < 0 {
		return nil
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
		return nil
	}

	var ads []types.AdSegment
	if err := json.Unmarshal([]byte(content[start:end+1]), &ads); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling ads JSON: %v\n", err)
		return nil
	}
	return ads
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
	current := ""
	for _, ch := range content {
		if ch == ',' || ch == '[' || ch == ']' || ch == '"' {
			if current != "" {
				keywords = append(keywords, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		keywords = append(keywords, current)
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

	result := ""
	for i, kw := range cleaned {
		if i > 0 {
			result += ", "
		}
		result += kw
	}
	return result
}
