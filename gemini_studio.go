package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type geminiStudioFileUploadResponse struct {
	File struct {
		Name     string `json:"name"`
		URI      string `json:"uri"`
		MimeType string `json:"mimeType"`
		State    string `json:"state"`
	} `json:"file"`
}

type geminiStudioGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func pipeMultipartAudio(pw *io.PipeWriter, mpw *multipart.Writer, localAudioPath, mimeType string) {
	var err error
	defer func() {
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="metadata"`)
	h.Set("Content-Type", "application/json; charset=UTF-8")
	part, err := mpw.CreatePart(h)
	if err != nil {
		return
	}
	meta := fmt.Sprintf(`{"file": {"display_name": %q}}`, filepath.Base(localAudioPath))
	if _, err = part.Write([]byte(meta)); err != nil {
		return
	}

	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filepath.Base(localAudioPath)))
	fileHeader.Set("Content-Type", mimeType)
	filePart, err := mpw.CreatePart(fileHeader)
	if err != nil {
		return
	}

	f, err := os.Open(localAudioPath)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err = io.Copy(filePart, f); err != nil {
		return
	}
	err = mpw.Close()
}

func uploadAudioToGeminiStudio(ctx context.Context, apiKey, localAudioPath string) (string, string, error) {
	var err error
	apiKey, err = validateGeminiKey(apiKey)
	if err != nil {
		return "", "", err
	}

	pr, pw := io.Pipe()
	mpw := multipart.NewWriter(pw)

	mimeType := "audio/mpeg"
	if strings.HasSuffix(strings.ToLower(localAudioPath), ".wav") {
		mimeType = "audio/wav"
	}

	go pipeMultipartAudio(pw, mpw, localAudioPath, mimeType)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/upload/v1beta/files?key=%s", apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, pr)
	if err != nil {
		return "", "", fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("X-Goog-Upload-Protocol", "multipart")
	req.Header.Set("Content-Type", mpw.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("gemini file upload failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("gemini file upload HTTP %d: %s", resp.StatusCode, string(body))
	}

	var res geminiStudioFileUploadResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", "", fmt.Errorf("failed to parse upload response: %w", err)
	}
	return res.File.URI, res.File.Name, nil
}

func deleteGeminiStudioFile(ctx context.Context, apiKey, fileName string) {
	if fileName == "" || apiKey == "" {
		return
	}
	var err error
	apiKey, err = validateGeminiKey(apiKey)
	if err != nil {
		return
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s?key=%s", fileName, apiKey)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

func callGeminiStudioProcessor(ctx context.Context, apiKey, modelName, fileURI string) (*geminiResponsePayload, error) {
	var err error
	apiKey, err = validateGeminiKey(apiKey)
	if err != nil {
		return nil, err
	}
	if modelName == "" {
		modelName = defaultGeminiModel
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

	reqPayload := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{
						"file_data": map[string]string{
							"mime_type": "audio/mpeg",
							"file_uri":  fileURI,
						},
					},
					{
						"text": geminiAdRemovalPrompt,
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"response_mime_type": "application/json",
			"temperature":        0.1,
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal studio request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create studio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini studio request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini studio generateContent HTTP %d: %s", resp.StatusCode, string(body))
	}

	return parseGeminiStudioResponse(body)
}

func parseGeminiStudioResponse(body []byte) (*geminiResponsePayload, error) {
	var res geminiStudioGenerateResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse studio response json: %w", err)
	}
	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty candidate in studio response: %s", string(body))
	}
	var sb strings.Builder
	for _, part := range res.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	return parseGeminiJSONString(sb.String())
}
