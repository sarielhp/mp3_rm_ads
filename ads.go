package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const systemPrompt = `You are an expert podcast editor assistant.
Your job is to analyze the timestamped transcript of a podcast episode and identify all advertisement segments, host-read sponsor plugs, promotional breaks, midroll/preroll ads, and sponsor call-outs.

Return ONLY a raw JSON array of objects with the exact start and end seconds of each ad segment, like this:
[
  {"start": 15.0, "end": 65.5, "reason": "Host read sponsor plug for VPN"},
  {"start": 1200.0, "end": 1290.0, "reason": "Midroll ad break"}
]

If NO ads or sponsor plugs are found, return an empty JSON array: []
Do not include markdown formatting or commentary outside the JSON array.`

const keywordExtractionPrompt = `You are a transcription assistant. Your job is to extract key topics, names, technical terms,
brand names, and unusual words from a podcast transcript segment.

Return ONLY a comma-separated list of 10-20 keywords/phrases (each 1-3 words).
Focus on: guest names, topic-specific jargon, product names, locations, and any words
that are unusual or easily misheard.

Keep each keyword short. Do not include markdown or commentary.`

type llmRequest struct {
	Model       string       `json:"model"`
	Messages    []llmMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmResponse struct {
	Choices []llmChoice `json:"choices"`
}

type llmChoice struct {
	Message llmMessage `json:"message"`
}

func callLLMChat(profile LLMProfile, sysPrompt, userPrompt string, maxTokens int, timeout time.Duration, quiet bool) (string, error) {
	payload := llmRequest{
		Model: profile.Model,
		Messages: []llmMessage{
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

	apiKey := profile.APIKey
	if apiKey == "" {
		apiKey = envOr("OPENROUTER_API_KEY", "")
	}
	apiKey, err = validateOpenRouterKey(profile, apiKey)
	if err != nil {
		return "", err
	}
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

	var llmResp llmResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return llmResp.Choices[0].Message.Content, nil
}

func detectAdsLLM(transcriptText string, profile LLMProfile) []AdSegment {
	if profile.URL == "" {
		return nil
	}
	userPrompt := fmt.Sprintf("Here is the podcast transcript with timestamps in seconds:\n\n%s", transcriptText)
	content, err := callLLMChat(profile, systemPrompt, userPrompt, 0, 30*time.Second, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during LLM ad detection: %v\n", err)
		return nil
	}
	return extractJSONArray(content)
}

func extractJSONArray(content string) []AdSegment {
	start := -1
	for i := 0; i < len(content); i++ {
		if content[i] == '[' {
			start = i
			break
		}
	}
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

	var ads []AdSegment
	if err := json.Unmarshal([]byte(content[start:end+1]), &ads); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling ads JSON: %v\n", err)
		return nil
	}
	return ads
}

func extractKeywordsLLM(transcriptText string, profile LLMProfile, quiet bool) string {
	userPrompt := fmt.Sprintf("Extract keywords from this podcast transcript segment:\n\n%s", transcriptText)
	content, err := callLLMChat(profile, keywordExtractionPrompt, userPrompt, 200, 60*time.Second, quiet)
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
		kw = trimSpace(kw)
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

func envOr(key, defaultVal string) string {
	return osGetenv(key, defaultVal)
}

func osGetenv(key, defaultVal string) string {
	val := envGetenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

var envGetenv = os.Getenv
