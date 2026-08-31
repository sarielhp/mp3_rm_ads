package main

import (
	"fmt"
)

func getProfileCost(profile LLMProfile) CostInfo {
	t := profile.Type
	u := profile.URL
	m := profile.Model

	if t == "ollama" || containsAny(u, []string{"11434", "localhost", "127.0.0.1"}) {
		return CostInfo{
			Type:     "Local",
			CostStr:  "Free ($0.00 / Local GPU)",
			Est1HStr: "$0.00",
		}
	}

	if containsAny(u, []string{"openrouter.ai"}) || t == "openrouter" {
		models := fetchOpenRouterModels()
		clean := cleanModelName(m)
		tokens := splitModelTokens(clean)

		var found *OpenRouterModel
		for i := range models {
			mid := cleanModelName(models[i].ID)
			if mid == clean || mid == m {
				found = &models[i]
				break
			}
		}
		if found == nil {
			for i := range models {
				mid := cleanModelName(models[i].ID)
				if allTokensMatch(mid, tokens) {
					found = &models[i]
					break
				}
			}
		}
		if found == nil {
			keyTerms := filterKeyTerms(tokens)
			if len(keyTerms) > 0 {
				for i := range models {
					mid := cleanModelName(models[i].ID)
					if allTokensMatch(mid, keyTerms) {
						found = &models[i]
						break
					}
				}
			}
		}

		if found != nil && found.Pricing.Prompt != "" {
			promptPrice := parseFloat(found.Pricing.Prompt)
			completionPrice := parseFloat(found.Pricing.Completion)
			in1M := roundFloat(promptPrice*1_000_000, 4)
			out1M := roundFloat(completionPrice*1_000_000, 4)
			est1H := roundFloat(13000*promptPrice+300*completionPrice, 4)
			return CostInfo{
				Type:     fmt.Sprintf("OpenRouter (%s)", found.ID),
				In1M:     in1M,
				Out1M:    out1M,
				CostStr:  fmt.Sprintf("In: $%.4f/1M, Out: $%.4f/1M", in1M, out1M),
				Est1HStr: fmt.Sprintf("~$%.4f / 1-hr episode", est1H),
			}
		}
	}

	return CostInfo{
		Type:     "Unknown",
		CostStr:  "Dynamic pricing unavailable (Check OpenRouter API)",
		Est1HStr: "N/A",
	}
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func cleanModelName(name string) string {
	result := name
	for _, prefix := range []string{"openrouter/", "~"} {
		if len(result) >= len(prefix) && result[:len(prefix)] == prefix {
			result = result[len(prefix):]
		}
	}
	for i := 0; i < len(result); i++ {
		if result[i] == ':' {
			result = result[:i]
			break
		}
	}
	return result
}

func splitModelTokens(name string) []string {
	var tokens []string
	current := ""
	for _, ch := range name {
		if ch == '/' || ch == '.' || ch == '_' || ch == '-' {
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
}

func allTokensMatch(id string, tokens []string) bool {
	for _, tok := range tokens {
		if !contains(id, tok) {
			return false
		}
	}
	return true
}

func filterKeyTerms(tokens []string) []string {
	keySet := map[string]bool{
		"sonnet": true, "haiku": true, "opus": true, "flash": true,
		"deepseek": true, "llama": true, "gemini": true, "qwen": true, "mistral": true,
	}
	var result []string
	for _, t := range tokens {
		if keySet[t] {
			result = append(result, t)
		}
	}
	return result
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func roundFloat(f float64, places int) float64 {
	pow := 1.0
	for i := 0; i < places; i++ {
		pow *= 10
	}
	return float64(int64(f*pow+0.5)) / pow
}
