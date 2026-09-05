package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/vertexai/genai"
)

func callGeminiAudioProcessor(ctx context.Context, projectID, location, gcsURI string) (*geminiResponsePayload, error) {
	client, err := genai.NewClient(ctx, projectID, location)
	if err != nil {
		return nil, fmt.Errorf("failed to create vertex ai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-flash")
	model.ResponseMIMEType = "application/json"
	model.SetTemperature(0.1)

	mimeType := "audio/mpeg"
	if strings.HasSuffix(strings.ToLower(gcsURI), ".wav") {
		mimeType = "audio/wav"
	}

	audioPart := genai.FileData{
		MIMEType: mimeType,
		FileURI:  gcsURI,
	}

	resp, err := model.GenerateContent(ctx, audioPart, genai.Text(geminiAdRemovalPrompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate content failed: %w", err)
	}

	return parseGeminiContentResponse(resp)
}

func parseGeminiContentResponse(resp *genai.GenerateContentResponse) (*geminiResponsePayload, error) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("empty response received from Gemini")
	}

	var jsonText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			jsonText += string(textPart)
		}
	}

	return parseGeminiJSONString(jsonText)
}

func parseGeminiJSONString(jsonText string) (*geminiResponsePayload, error) {
	trimmed := strings.TrimSpace(jsonText)
	if trimmed == "" {
		return nil, fmt.Errorf("empty JSON text received from Gemini")
	}
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			end := len(lines)
			if strings.HasPrefix(lines[len(lines)-1], "```") {
				end = len(lines) - 1
			}
			trimmed = strings.TrimSpace(strings.Join(lines[1:end], "\n"))
		}
	}
	var payload geminiResponsePayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini JSON: %w (raw response: %s)", err, jsonText)
	}
	return &payload, nil
}
