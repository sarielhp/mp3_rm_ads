package mailclient

import (
	"fmt"
	"strings"
)

func ResolveLabel(client LabelReader, inputLabel string) (string, error) {
	if inputLabel == "" {
		return "", nil
	}

	matches, err := client.GetMatchingLabels(inputLabel)
	if err == nil && len(matches) > 0 {
		return inputLabel, nil
	}

	allLabels, err := client.GetMatchingLabels("")
	if err != nil {
		return inputLabel, err
	}

	hasSublabel := func(label string) bool {
		prefix := strings.ToLower(label) + "/"
		for _, l := range allLabels {
			if strings.HasPrefix(strings.ToLower(l), prefix) {
				return true
			}
		}
		return false
	}

	var candidates []string
	inputLower := strings.ToLower(inputLabel)
	suffixMatch := "/" + inputLower

	for _, label := range allLabels {
		labelLower := strings.ToLower(label)
		if labelLower == inputLower || strings.HasSuffix(labelLower, suffixMatch) {
			if !hasSublabel(label) {
				candidates = append(candidates, label)
			}
		}
	}

	if len(candidates) == 1 {
		fmt.Printf("[*] Using label: %s\n", candidates[0])
		return candidates[0], nil
	}

	return inputLabel, nil
}
