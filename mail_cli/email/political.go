package email

import "strings"

var politicalInstantBlocks = map[string]string{
	"actblue.com":                          "ActBlue donation link",
	"winred.com":                           "WinRed donation link",
	"paid for by":                          "Political PAC disclosure ('paid for by')",
	"contributions are not tax deductible": "Non-tax-deductible disclosure",
	"contributions or gifts to":            "Donation policy statement",
	"not authorized by any candidate":      "Independent expenditure statement",
	"political action committee":           "PAC reference",
}

var politicalContext = map[string]string{
	"democrat":    "Democrat context",
	"republican":  "Republican context",
	"gop":         "GOP context",
	"dnc":         "DNC context",
	"rnc":         "RNC context",
	"super pac":   "Super PAC reference",
	"trump":       "Trump campaign",
	"harris":      "Harris campaign",
	"biden":       "Biden reference",
	"obama":       "Obama reference",
	"congress":    "Congress context",
	"senate":      "Senate context",
	"election":    "Election reference",
	"campaign":    "Campaign reference",
	"ballot":      "Ballot reference",
	"nominee":     "Nominee reference",
	"polling":     "Polling reference",
	"white house": "White House context",
}

var politicalDonationWords = map[string]string{
	"donate":        "Donate call-to-action",
	"donation":      "Donation request",
	"contribution":  "Contribution request",
	"contribute":    "Contribute call-to-action",
	"fundraiser":    "Fundraiser event",
	"fundraising":   "Fundraising campaign",
	"pitch in":      "Pitch-in appeal",
	"matching gift": "Matching gift appeal",
	"dollar match":  "Dollar match campaign",
	"chip in":       "Chip-in appeal",
}

var politicalUrgencyWords = map[string]string{
	"urgent":          "Urgency context",
	"critical":        "Critical alert",
	"deadline":        "Deadline alert",
	"desperate":       "Desperation appeal",
	"patriot":         "Patriot appeal",
	"double match":    "Double match",
	"triple match":    "Triple match",
	"quadruple match": "Quadruple match",
	"expire":          "Expiration alert",
	"suspended":       "Suspension alert",
}

func DetectPolitical(subject, body string) (bool, float64, []string) {
	subjectLower := strings.ToLower(subject)
	bodyLower := strings.ToLower(body)

	const maxTextLen = 8192
	fullText := subjectLower + " " + bodyLower
	if len(fullText) > maxTextLen {
		fullText = fullText[:maxTextLen]
	}

	var score float64
	var triggered []string

	for keyword, reason := range politicalInstantBlocks {
		if strings.Contains(fullText, keyword) {
			score += 20.0
			triggered = append(triggered, reason)
		}
	}

	for key, label := range politicalContext {
		if strings.Contains(fullText, key) {
			score += 4.0
			triggered = append(triggered, label)
		}
	}

	for key, label := range politicalDonationWords {
		if strings.Contains(fullText, key) {
			score += 4.0
			triggered = append(triggered, label)
		}
	}

	for key, label := range politicalUrgencyWords {
		if strings.Contains(fullText, key) {
			score += 2.0
			triggered = append(triggered, label)
		}
	}

	hasContext := false
	for key := range politicalInstantBlocks {
		if strings.Contains(fullText, key) {
			hasContext = true
			break
		}
	}
	if !hasContext {
		for key := range politicalContext {
			if strings.Contains(fullText, key) {
				hasContext = true
				break
			}
		}
	}

	isPolitical := score >= 10.0 && hasContext
	return isPolitical, score, triggered
}

// IsSafeToAutoBlacklist evaluates if a political sender is safe to blacklist based on conservative criteria.
func IsSafeToAutoBlacklist(senderEmail, listUnsubscribe string, score float64) bool {
	if listUnsubscribe == "" {
		return false
	}
	if score < 15.0 {
		return false
	}

	parts := strings.Split(senderEmail, "@")
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(parts[1]))

	publicDomains := map[string]bool{
		"gmail.com":      true,
		"yahoo.com":      true,
		"outlook.com":    true,
		"hotmail.com":    true,
		"proton.me":      true,
		"protonmail.com": true,
		"icloud.com":     true,
		"aol.com":        true,
		"zoho.com":       true,
		"gmx.com":        true,
		"mail.com":       true,
		"yandex.com":     true,
	}

	return !publicDomains[domain]
}
