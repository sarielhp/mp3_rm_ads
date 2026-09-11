package gemini

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

	"abs/pkg/types"
	"abs/pkg/util"
)

const defaultGeminiModel = "gemini-flash-latest"

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

func validateKey(apiKey string) (string, error) {
	if apiKey == "" || util.IsZeroedKey(apiKey) {
		if apiKey != "" {
			_ = util.ZeroWipeKey(apiKey)
		}
		return "", fmt.Errorf("gemini API key is disabled in configuration")
	}
	return apiKey, nil
}

func PipeMultipartAudio(pw *io.PipeWriter, mpw *multipart.Writer, localAudioPath, mimeType string) {
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

func UploadAudioToGeminiStudio(ctx context.Context, apiKey, localAudioPath string) (string, string, error) {
	var err error
	apiKey, err = validateKey(apiKey)
	if err != nil {
		return "", "", err
	}

	pr, pw := io.Pipe()
	mpw := multipart.NewWriter(pw)

	mimeType := "audio/mpeg"
	if strings.HasSuffix(strings.ToLower(localAudioPath), ".wav") {
		mimeType = "audio/wav"
	}

	go PipeMultipartAudio(pw, mpw, localAudioPath, mimeType)

	url := "https://generativelanguage.googleapis.com/upload/v1beta/files"
	req, err := http.NewRequestWithContext(ctx, "POST", url, pr)
	if err != nil {
		return "", "", fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("x-goog-api-key", apiKey)
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
		return "", "", fmt.Errorf("gemini file upload HTTP %d: %s", resp.StatusCode, FormatGeminiErrorBody(body))
	}

	var res geminiStudioFileUploadResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", "", fmt.Errorf("failed to parse upload response: %w", err)
	}
	return res.File.URI, res.File.Name, nil
}

func DeleteGeminiStudioFile(ctx context.Context, apiKey, fileName string) {
	if fileName == "" || apiKey == "" {
		return
	}
	var err error
	apiKey, err = validateKey(apiKey)
	if err != nil {
		return
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s", fileName)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("x-goog-api-key", apiKey)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

type geminiErrorDetail struct {
	Type     string            `json:"@type"`
	Reason   string            `json:"reason"`
	Domain   string            `json:"domain"`
	Metadata map[string]string `json:"metadata"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int                 `json:"code"`
		Message string              `json:"message"`
		Status  string              `json:"status"`
		Details []geminiErrorDetail `json:"details"`
	} `json:"error"`
}

func CallGeminiStudioProcessor(ctx context.Context, apiKey, modelName, fileURI string) (*types.GeminiResponsePayload, error) {
	var err error
	apiKey, err = validateKey(apiKey)
	if err != nil {
		return nil, err
	}
	if modelName == "" {
		modelName = defaultGeminiModel
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", modelName)

	reqBytes, err := buildStudioGeneratePayload(fileURI)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		body, statusCode, err := executeStudioRequest(ctx, client, url, apiKey, reqBytes)
		if err != nil {
			lastErr = fmt.Errorf("gemini studio request failed: %w", err)
		} else if statusCode == http.StatusOK {
			return ParseGeminiStudioResponse(body)
		} else {
			errMsg := FormatGeminiErrorBody(body)
			lastErr = fmt.Errorf("gemini studio generateContent HTTP %d: %s", statusCode, errMsg)
			if statusCode == http.StatusTooManyRequests {
				if IsGeminiDailyQuotaExhausted(body) {
					TripCircuitBreaker(errMsg, DefaultDailyQuotaCooldown)
					return nil, lastErr
				}
				if attempt == maxAttempts {
					TripCircuitBreaker(errMsg, DefaultRateLimitCooldown)
				}
			} else if statusCode != http.StatusServiceUnavailable && statusCode != http.StatusGatewayTimeout {
				return nil, lastErr
			}
		}

		if attempt < maxAttempts {
			delay := time.Duration(attempt*2) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return nil, lastErr
}

func buildStudioGeneratePayload(fileURI string) ([]byte, error) {
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
						"text": types.GeminiAdRemovalPrompt,
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"response_mime_type": "application/json",
			"temperature":        0.1,
		},
	}
	data, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal studio request: %w", err)
	}
	return data, nil
}

func executeStudioRequest(ctx context.Context, client *http.Client, url, apiKey string, reqBytes []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create studio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func FormatGeminiErrorBody(body []byte) string {
	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		msg := strings.TrimSpace(errResp.Error.Message)
		if quotaInfo := extractGeminiQuotaDetails(&errResp); quotaInfo != "" {
			return fmt.Sprintf("%s [%s]", msg, quotaInfo)
		}
		if errResp.Error.Status != "" {
			return fmt.Sprintf("%s (%s)", msg, errResp.Error.Status)
		}
		return msg
	}
	return strings.TrimSpace(string(body))
}

func extractGeminiQuotaDetails(errResp *geminiErrorResponse) string {
	for _, d := range errResp.Error.Details {
		if len(d.Metadata) == 0 {
			continue
		}
		limit := d.Metadata["quota_limit"]
		val := d.Metadata["quota_limit_value"]
		consumer := d.Metadata["consumer"]

		var parts []string
		if limit != "" {
			if strings.Contains(limit, "PerDay") || val == "1500" {
				parts = append(parts, "Free Tier: Daily quota exhausted (1,500 req/day limit)")
			} else if strings.Contains(limit, "PerMinute") || val == "15" {
				parts = append(parts, "Free Tier: Rate limit exceeded (15 req/min limit)")
			} else {
				parts = append(parts, fmt.Sprintf("Quota: %s", limit))
			}
		}
		if val != "" && !strings.Contains(limit, "PerDay") && !strings.Contains(limit, "PerMinute") {
			parts = append(parts, fmt.Sprintf("Limit: %s", val))
		}
		if consumer != "" {
			parts = append(parts, fmt.Sprintf("Project: %s", strings.TrimPrefix(consumer, "projects/")))
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}
	return ""
}

func IsGeminiDailyQuotaExhausted(body []byte) bool {
	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return false
	}
	for _, d := range errResp.Error.Details {
		limit := d.Metadata["quota_limit"]
		val := d.Metadata["quota_limit_value"]
		if strings.Contains(limit, "PerDay") || val == "1500" {
			return true
		}
	}
	msg := strings.ToLower(errResp.Error.Message)
	return strings.Contains(msg, "exceeded your current quota") && !strings.Contains(msg, "per minute")
}

func ParseGeminiStudioResponse(body []byte) (*types.GeminiResponsePayload, error) {
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
	return ParseGeminiJSONString(sb.String())
}
