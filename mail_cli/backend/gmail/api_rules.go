package gmail

import (
	"bytes"
	"fmt"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mime"
	"net/mail"
	"strings"

	gmailapi "google.golang.org/api/gmail/v1"
)

// checkAndApplyRulesREST checks rules against downloaded emails and applies labels via the Gmail API.
func checkAndApplyRulesREST(messageIDs []string, config *Config, sourceLabelName string, cacheSubdir string) ([]string, error) {
	if len(messageIDs) == 0 || len(config.Rules) == 0 {
		return messageIDs, nil
	}

	dec := new(mime.WordDecoder)

	ruleMatches := make(map[Rule][]string)
	matchedIDs := make(map[string]bool)

	for _, id := range messageIDs {
		rawBytes, err := msg.Read(config.DownloadDir, id)
		if err != nil {
			if config.Verbose {
				fmt.Printf("%s Warning: Failed to open cached email for rule checking (ID %s): %v\n", app.PrefixWarn, id, err)
			}
			continue
		}
		msg, err := mail.ReadMessage(bytes.NewReader(rawBytes))
		if err != nil {
			if config.Verbose {
				fmt.Printf("%s Warning: Failed to parse cached email headers for rule checking (ID %s): %v\n", app.PrefixWarn, id, err)
			}
			continue
		}

		sender := email.ParseEmailAddress(msg.Header.Get("From"))
		subject := email.DecodeHeader(dec, msg.Header.Get("Subject"))

		if rule := cfg_acc.MatchRules(config.Rules, sender, subject); rule != nil {
			ruleMatches[*rule] = append(ruleMatches[*rule], id)
			matchedIDs[id] = true
			if config.Verbose {
				fmt.Printf("%s Rule Match (ID %s, Subject: %q) matches rule -> label %q\n", app.PrefixInfo, id, subject, rule.Label)
			}
		}
	}

	if len(matchedIDs) == 0 {
		return messageIDs, nil
	}

	srv, err := GetGmailService(config)
	if err != nil {
		return messageIDs, err
	}

	// Fetch existing labels
	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return messageIDs, fmt.Errorf("failed to list Gmail labels: %w", err)
	}

	labelNameToID := make(map[string]string)
	labelIDToName := make(map[string]string)
	for _, l := range labelsRes.Labels {
		lowerName := strings.ToLower(l.Name)
		labelNameToID[lowerName] = l.Id
		labelIDToName[l.Id] = l.Name
	}

	// Resolve the source label ID
	sourceLabelID := ""
	if strings.EqualFold(sourceLabelName, "inbox") {
		sourceLabelID = "INBOX"
	} else {
		if id, ok := labelNameToID[strings.ToLower(sourceLabelName)]; ok {
			sourceLabelID = id
		} else {
			return messageIDs, fmt.Errorf("source label %q not found on Gmail", sourceLabelName)
		}
	}

	for rule, ids := range ruleMatches {
		targetLabelID, err := ensureLabelHierarchyREST(srv, rule.Label, labelNameToID, labelIDToName)
		if err != nil {
			return messageIDs, err
		}

		ruleDesc := rule.Sender
		if rule.Subject != "" {
			ruleDesc = fmt.Sprintf("subject: %s*", rule.Subject)
		}

		removeLabels := []string{sourceLabelID}
		if sourceLabelID == "INBOX" {
			if recID, ok := labelNameToID["received"]; ok {
				removeLabels = append(removeLabels, recID)
			}
		} else {
			removeLabels = append(removeLabels, "INBOX")
		}

		// Ensure targetLabelID is not in removeLabels to prevent Gmail API Error 400: Cannot both add and remove the same label
		var finalRemoveLabels []string
		for _, rl := range removeLabels {
			if rl != targetLabelID {
				finalRemoveLabels = append(finalRemoveLabels, rl)
			}
		}

		var affectedIDs []string
		for _, id := range ids {
			msgMeta, errMeta := srv.Users.Messages.Get("me", id).Format("minimal").Do()
			if errMeta != nil {
				affectedIDs = append(affectedIDs, id)
				continue
			}

			hasTargetLabel := false
			for _, lid := range msgMeta.LabelIds {
				if lid == targetLabelID {
					hasTargetLabel = true
					break
				}
			}

			hasLabelsToRemove := false
			for _, rlid := range finalRemoveLabels {
				for _, lid := range msgMeta.LabelIds {
					if lid == rlid {
						hasLabelsToRemove = true
						break
					}
				}
				if hasLabelsToRemove {
					break
				}
			}

			if !hasTargetLabel || hasLabelsToRemove {
				affectedIDs = append(affectedIDs, id)
			} else {
				matchedIDs[id] = false
				if config.Verbose {
					fmt.Printf("%s Rule match has 0 effect (already labeled %q and not in removable labels) for ID %s. Keeping message as-is.\n", app.PrefixInfo, rule.Label, id)
				}
			}
		}

		if len(affectedIDs) == 0 {
			continue
		}

		if rule.Exported {
			fmt.Printf("%s Warning: Applying local rule for '%s' -> %q, which has already been exported to Gmail server filters!\n",
				app.PrefixWarn, ruleDesc, rule.Label)
		}
		fmt.Printf("%s Applying rule: labeling %d emails matching '%s' with %q and archiving them from %s\n",
			app.PrefixSuccess, len(affectedIDs), ruleDesc, rule.Label, sourceLabelName)

		err = srv.Users.Messages.BatchModify("me", &gmailapi.BatchModifyMessagesRequest{
			Ids:            affectedIDs,
			AddLabelIds:    []string{targetLabelID},
			RemoveLabelIds: finalRemoveLabels,
		}).Do()
		if err != nil {
			return messageIDs, fmt.Errorf("failed to batch modify messages for rule %q: %w", ruleDesc, err)
		}
	}

	var remainingIDs []string
	for _, id := range messageIDs {
		if !matchedIDs[id] {
			remainingIDs = append(remainingIDs, id)
		}
	}

	return remainingIDs, nil
}

// exportRulesToGmailREST exports local rules from config.json to Gmail filters.
func updateLocalRulesInConfig(config *Config, rules []Rule) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}
	targetAcc.Rules = rules
	return cfg_g.SaveConfigFile(configPath, fc)
}

// findMinimumUniquePrefixLength finds the smallest prefix length n >= 1
// such that all prefixes of length n of the given IDs are unique.
func findMinimumUniquePrefixLength(ids []string) int {
	if len(ids) <= 1 {
		return 1
	}

	maxLen := 0
	for _, id := range ids {
		if len(id) > maxLen {
			maxLen = len(id)
		}
	}

	for n := 1; n <= maxLen; n++ {
		seen := make(map[string]bool)
		unique := true
		for _, id := range ids {
			pref := id
			if len(id) > n {
				pref = id[:n]
			}
			if seen[pref] {
				unique = false
				break
			}
			seen[pref] = true
		}
		if unique {
			return n
		}
	}
	return maxLen
}

// listGmailFiltersREST fetches and prints all remote Gmail filters with detailed descriptions.

func listGmailFiltersREST(config *Config) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Fetching existing labels from Gmail...\n", app.PrefixInfo)
	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return HandleScopeError(config, fmt.Errorf("failed to list Gmail labels: %w", err))
	}

	labelIDToName := make(map[string]string)
	for _, l := range labelsRes.Labels {
		labelIDToName[l.Id] = l.Name
	}

	fmt.Printf("%s Fetching existing filters from Gmail...\n", app.PrefixInfo)
	filtersRes, err := srv.Users.Settings.Filters.List("me").Do()
	if err != nil {
		return HandleScopeError(config, fmt.Errorf("failed to list Gmail filters: %w", err))
	}

	app.ColorBoldCyan.Println("\n======================================================================")
	app.ColorBoldCyan.Println("                         REMOTE GMAIL FILTERS                         ")
	app.ColorBoldCyan.Println("======================================================================")

	if len(filtersRes.Filter) == 0 {
		fmt.Println("  No remote filters found on Gmail.")
	} else {
		// Collect filter IDs to calculate minimum unique prefix length
		var filterIDs []string
		for _, f := range filtersRes.Filter {
			filterIDs = append(filterIDs, f.Id)
		}
		prefixLen := findMinimumUniquePrefixLength(filterIDs)
		for i, f := range filtersRes.Filter {
			// Format criteria
			var critParts []string
			if f.Criteria != nil {
				if f.Criteria.From != "" {
					critParts = append(critParts, fmt.Sprintf("From: %s", f.Criteria.From))
				}
				if f.Criteria.To != "" {
					critParts = append(critParts, fmt.Sprintf("To: %s", f.Criteria.To))
				}
				if f.Criteria.Subject != "" {
					critParts = append(critParts, fmt.Sprintf("Subject: %s", f.Criteria.Subject))
				}
				if f.Criteria.Query != "" {
					critParts = append(critParts, fmt.Sprintf("Has words: %s", f.Criteria.Query))
				}
				if f.Criteria.NegatedQuery != "" {
					critParts = append(critParts, fmt.Sprintf("Doesn't have: %s", f.Criteria.NegatedQuery))
				}
			}
			critDesc := strings.Join(critParts, ", ")
			if critDesc == "" {
				critDesc = "Any message"
			}

			// Format action
			actionDesc := "No action defined"
			if f.Action != nil {
				var actions []string
				if len(f.Action.AddLabelIds) > 0 {
					var adds []string
					for _, id := range f.Action.AddLabelIds {
						if name, ok := labelIDToName[id]; ok {
							adds = append(adds, name)
						} else {
							adds = append(adds, id)
						}
					}
					actions = append(actions, fmt.Sprintf("Add labels: %s", strings.Join(adds, ", ")))
				}
				if len(f.Action.RemoveLabelIds) > 0 {
					var removes []string
					for _, id := range f.Action.RemoveLabelIds {
						if name, ok := labelIDToName[id]; ok {
							removes = append(removes, name)
						} else {
							removes = append(removes, id)
						}
					}
					actions = append(actions, fmt.Sprintf("Remove labels: %s", strings.Join(removes, ", ")))
				}
				if f.Action.Forward != "" {
					actions = append(actions, fmt.Sprintf("Forward to: %s", f.Action.Forward))
				}
				if len(actions) > 0 {
					actionDesc = strings.Join(actions, "; ")
				}
			}

			displayID := f.Id
			if len(displayID) > prefixLen {
				displayID = displayID[:prefixLen] + "..."
			}
			fmt.Printf("  [%d] Filter ID: %s\n", i+1, displayID)
			fmt.Printf("      Matches: %s\n", critDesc)
			fmt.Printf("      Action:  %s\n\n", actionDesc)
		}
	}
	app.ColorBoldCyan.Println("======================================================================")
	return nil
}
