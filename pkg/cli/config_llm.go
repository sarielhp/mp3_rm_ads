package cli

import (
	"fmt"

	"abs/pkg/config"
	"abs/pkg/util"
)

func listProfiles(cfg Config) {
	activeProfile, _ := config.SelectLLMProfile(&cfg, "")
	activeID := activeProfile.ID
	fmt.Printf("\n%s\n", util.RepeatStr("=", 70))
	fmt.Println("AVAILABLE LLM PROFILES & PRICING:")
	fmt.Printf("%s\n", util.RepeatStr("=", 70))

	for _, p := range cfg.Profiles {
		isDefault := p.ID == activeID
		defaultBadge := ""
		if isDefault {
			defaultBadge = " [DEFAULT]"
		}
		hasKey := ""
		if p.APIKey != "" {
			hasKey = " (Key set)"
		}
		costInfo := config.GetProfileCost(p)

		headerStr := fmt.Sprintf("  [%d] %s", p.ID, p.Name)
		headerStr += defaultBadge
		if isDefault {
			headerStr = util.BoldGreen(headerStr)
		}
		fmt.Println(headerStr)
		fmt.Printf("      - Model:     %s\n", p.Model)
		fmt.Printf("      - Type:      %s%s\n", p.Type, hasKey)
		fmt.Printf("      - Pricing:   %s\n", costInfo.CostStr)
		fmt.Printf("      - Est. 1-Hr: %s\n", costInfo.Est1HStr)
		fmt.Printf("      - URL:       %s\n", p.URL)
		fmt.Println()
	}
	fmt.Printf("%s\n\n", util.RepeatStr("=", 70))
}

func setDefaultProfile(cfg *Config, targetID int) {
	if err := config.SetDefaultProfile(cfg, targetID); err != nil {
		fatalError("Error: Profile ID [%d] not found in configuration.\n", targetID)
		return
	}
	_ = config.SaveConfig(cfg)
	for _, p := range cfg.Profiles {
		if p.ID == targetID {
			fmt.Printf("Default LLM profile updated to [%d] %s\n", targetID, p.Name)
			return
		}
	}
}
