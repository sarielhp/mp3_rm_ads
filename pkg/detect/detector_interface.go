package detect

import (
	"context"
	"fmt"
	"time"

	"abs/pkg/types"
)

// AdDetector defines the interface for detecting advertisements in a transcript.
type AdDetector interface {
	DetectAds(ctx context.Context, transcriptText string) ([]types.AdSegment, error)
}

// LLMAdDetector implements AdDetector using a configured LLM profile.
type LLMAdDetector struct {
	Profile types.LLMProfile
	APIKey  string
	Timeout time.Duration
}

func NewLLMAdDetector(profile types.LLMProfile, apiKey string, timeout time.Duration) *LLMAdDetector {
	if timeout <= 0 {
		timeout = DefaultLLMTimeout
	}
	return &LLMAdDetector{
		Profile: profile,
		APIKey:  apiKey,
		Timeout: timeout,
	}
}

func (d *LLMAdDetector) DetectAds(ctx context.Context, transcriptText string) ([]types.AdSegment, error) {
	return DetectAdsLLMTimeout(transcriptText, d.Profile, d.APIKey, d.Timeout)
}

// ConfirmingDetector wraps an AdDetector and confirms empty results before accepting them.
type ConfirmingDetector struct {
	Base          AdDetector
	Confirmations int
}

func NewConfirmingDetector(base AdDetector, confirmations int) *ConfirmingDetector {
	if confirmations <= 0 {
		confirmations = EmptyResultConfirmations
	}
	return &ConfirmingDetector{
		Base:          base,
		Confirmations: confirmations,
	}
}

func (c *ConfirmingDetector) DetectAds(ctx context.Context, transcriptText string) ([]types.AdSegment, error) {
	segs, err := c.Base.DetectAds(ctx, transcriptText)
	if err != nil || len(segs) > 0 {
		return segs, err
	}
	for i := 0; i < c.Confirmations; i++ {
		retry, retryErr := c.Base.DetectAds(ctx, transcriptText)
		if retryErr != nil {
			return nil, fmt.Errorf("confirmation attempt %d failed: %w", i+1, retryErr)
		}
		if len(retry) > 0 {
			return retry, nil
		}
	}
	return segs, nil
}
