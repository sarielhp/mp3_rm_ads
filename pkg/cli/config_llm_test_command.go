package cli

import (
	"abs/pkg/config"
	"abs/pkg/detect"
	"abs/pkg/util"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/sarielhp/clihelp"
)

func buildConfigLLMTestSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "test",
		Description: "Test an LLM profile with a sample ad-detection request",
		UsageLine:   "abs config llm test <id>",
		Args:        clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "config"
			opts.ConfigCmd, opts.ConfigVal = "llm-test", ctx.Args[0]
			return nil
		},
	}
}

func testLLMProfile(cfg Config, value string) error {
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return fmt.Errorf("invalid LLM profile ID %q", value)
	}
	for _, profile := range cfg.Profiles {
		if profile.ID != id {
			continue
		}
		fmt.Println("\n" + util.BoldCyan(fmt.Sprintf("Testing LLM [%d]: %s", id, profile.Name)))
		detect.AnnounceAdDetection(profile, false)
		started := time.Now()
		if err := probeLLMProfile(cfg, profile); err != nil {
			fmt.Println("\n" + util.BoldRed(fmt.Sprintf("LLM [%d] test FAILED", id)) + "\n")
			return fmt.Errorf("LLM [%d]: %w", id, err)
		}
		fmt.Println(util.BoldGreen(fmt.Sprintf("LLM [%d] test PASSED: valid ad-detection response (%s)", id, time.Since(started).Round(time.Millisecond))))
		return nil
	}
	return fmt.Errorf("LLM profile [%d] not found", id)
}

func probeLLMProfile(cfg Config, profile LLMProfile) error {
	if strings.TrimSpace(profile.URL) == "" || strings.TrimSpace(profile.Model) == "" {
		return fmt.Errorf("profile requires both an API URL and model")
	}
	key := config.ResolveOpenRouterAPIKey(profile, &cfg)
	key, err := config.ValidateOpenRouterKey(profile, key, cfg.IsOpenRouterAPIKeyEnabled())
	if err != nil {
		return err
	}
	const sample = "[0.0s -> 10.0s] Welcome to our discussion of astronomy.\n[10.0s -> 20.0s] This episode is sponsored by Example Coffee. Visit example.com and use code PODCAST for ten percent off.\n[20.0s -> 30.0s] Back to our discussion of distant stars."
	ads, err := detect.DetectAdsLLMTimeout(sample, profile, key, 15*time.Second)
	if err != nil {
		message := err.Error()
		if key != "" {
			message = strings.ReplaceAll(message, key, "[redacted]")
		}
		return fmt.Errorf("%s", message)
	}
	for _, ad := range ads {
		if math.IsNaN(ad.Start) || math.IsNaN(ad.End) || math.IsInf(ad.Start, 0) || math.IsInf(ad.End, 0) || ad.Start < 0 || ad.End <= ad.Start || ad.End > 30 {
			return fmt.Errorf("API returned invalid ad timestamps: %g–%g", ad.Start, ad.End)
		}
	}
	return nil
}
