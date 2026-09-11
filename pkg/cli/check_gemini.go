package cli

import (
	"context"
	"fmt"
	"time"

	"abs/pkg/gemini"
	"abs/pkg/util"
)

func testGeminiAPI(cfg *Config, quiet bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if !quiet {
		fmt.Printf("Testing Gemini API connection (model: %s)...\n", cfg.GetGeminiModel())
	}

	res, err := gemini.ProbeGeminiAPI(ctx, cfg)
	if err == nil {
		if !quiet {
			fmt.Printf("   %s Gemini API key is valid and responsive!\n", util.BoldGreen("✓"))
			fmt.Printf("     • Model:   %s\n", res.Model)
			fmt.Printf("     • Latency: %s\n", res.Latency.Round(time.Millisecond))
			fmt.Printf("     • Status:  Ready for speculative transcription\n")
		}
		return nil
	}

	if !quiet {
		fmt.Printf("   %s Gemini API check failed: %v\n", util.BoldRed("✗"), err)
		if res != nil && res.Tier != "" {
			fmt.Printf("     • Plan:    %s\n", util.BoldYellow(res.Tier))
		}
		if res != nil && res.ProjectID != "" {
			fmt.Printf("     • Project: %s\n", res.ProjectID)
		}
	}
	return fmt.Errorf("gemini check failed: %w", err)
}
