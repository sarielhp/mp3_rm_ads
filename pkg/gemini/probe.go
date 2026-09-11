package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"abs/pkg/config"
	"abs/pkg/types"
)

type GeminiProbeResult struct {
	Model     string
	Latency   time.Duration
	Tier      string
	ProjectID string
	QuotaInfo string
}

func buildProbeRequest(ctx context.Context, modelName, apiKey string) (*http.Request, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", modelName)
	reqPayload := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": "ping"}}},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 1,
		},
	}
	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal probe payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)
	return req, nil
}

func ProbeGeminiAPI(ctx context.Context, cfg *types.Config) (*GeminiProbeResult, error) {
	apiKey := config.ResolveGeminiAPIKey(cfg)
	if apiKey == "" {
		return nil, fmt.Errorf("no Gemini API key found (checked config.json, GEMINI_API_KEY, and ~/.config/auth/)")
	}

	modelName := cfg.GetGeminiModel()
	if modelName == "" {
		modelName = defaultGeminiModel
	}

	req, err := buildProbeRequest(ctx, modelName, apiKey)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("gemini network error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		ResetCircuitBreaker()
		return &GeminiProbeResult{
			Model:   modelName,
			Latency: latency,
			Tier:    "Active",
		}, nil
	}

	res := inspectProbeError(body, modelName, latency)
	if resp.StatusCode == http.StatusTooManyRequests {
		if IsGeminiDailyQuotaExhausted(body) {
			TripCircuitBreaker(res.QuotaInfo, DefaultDailyQuotaCooldown)
		} else {
			TripCircuitBreaker(res.QuotaInfo, DefaultRateLimitCooldown)
		}
	}
	return res, fmt.Errorf("HTTP %d: %s", resp.StatusCode, res.QuotaInfo)
}

func inspectProbeError(body []byte, modelName string, latency time.Duration) *GeminiProbeResult {
	errMsg := FormatGeminiErrorBody(body)
	var errResp geminiErrorResponse
	_ = json.Unmarshal(body, &errResp)

	res := &GeminiProbeResult{
		Model:     modelName,
		Latency:   latency,
		QuotaInfo: errMsg,
	}
	for _, d := range errResp.Error.Details {
		if d.Metadata["consumer"] != "" {
			res.ProjectID = strings.TrimPrefix(d.Metadata["consumer"], "projects/")
		}
		limit := d.Metadata["quota_limit"]
		val := d.Metadata["quota_limit_value"]
		if strings.Contains(limit, "PerDay") || val == "1500" {
			res.Tier = "Free Tier (Daily Limit Exhausted)"
		} else if strings.Contains(limit, "PerMinute") || val == "15" {
			res.Tier = "Free Tier (Rate Limit Exceeded)"
		}
	}
	if res.Tier == "" && strings.Contains(strings.ToLower(errMsg), "quota") {
		res.Tier = "Free Tier (Quota Exceeded)"
	}
	return res
}
