package detect

import (
	"fmt"
	"net/url"
	"pod/pkg/types"
	"pod/pkg/util"
)

func AnnounceAdDetection(profile types.LLMProfile, quiet bool) {
	if quiet || profile.URL == "" {
		return
	}
	service := profile.Type
	if service == "" {
		service = "AI service"
	}
	if u, err := url.Parse(profile.URL); err == nil && u.Hostname() != "" {
		service += " on " + u.Hostname()
	}
	fmt.Println("\n" + util.BoldCyan(fmt.Sprintf("Ad detection: %s (model: %s)", service, profile.Model)))
}
