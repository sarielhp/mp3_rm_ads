package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"pod/pkg/gemini"
	"pod/pkg/util"
)

func testGeminiAPI(w io.Writer, cfg *Config, quiet bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if !quiet {
		fmt.Fprintf(w, "Testing Gemini API connection (model: %s)...\n", cfg.GetGeminiModel())
	}

	res, err := gemini.ProbeGeminiAPI(ctx, cfg)
	if err == nil {
		if !quiet {
			fmt.Fprintf(w, "   %s Gemini API key is valid and responsive!\n", util.BoldGreen("✓"))
			fmt.Fprintf(w, "     • Model:   %s\n", res.Model)
			fmt.Fprintf(w, "     • Latency: %s\n", res.Latency.Round(time.Millisecond))
			fmt.Fprintf(w, "     • Status:  Ready for speculative transcription\n")
		}
		return nil
	}

	if !quiet {
		fmt.Fprintf(w, "   %s Gemini API check failed: %v\n", util.BoldRed("✗"), err)
		if res != nil && res.Tier != "" {
			fmt.Fprintf(w, "     • Plan:    %s\n", util.BoldYellow(res.Tier))
		}
		if res != nil && res.ProjectID != "" {
			fmt.Fprintf(w, "     • Project: %s\n", res.ProjectID)
		}
	}
	return fmt.Errorf("gemini check failed: %w", err)
}
