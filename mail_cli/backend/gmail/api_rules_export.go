package gmail

import (
	"fmt"
	"mail_cli/app"
	"strings"

	gmailapi "google.golang.org/api/gmail/v1"
)

// checkAndApplyRulesREST checks rules against downloaded emails and applies labels via the Gmail API.
func exportRulesToGmailREST(config *Config) error {
	if len(config.Rules) == 0 {
		fmt.Printf("%s No rules defined in config.json. Nothing to export.\n", app.PrefixInfo)
		return nil
	}

	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Fetching existing labels from Gmail...\n", app.PrefixInfo)
	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return HandleScopeError(config, fmt.Errorf("failed to list Gmail labels: %w", err))
	}

	// Build a map of lowercase label name to exact casing name on Gmail
	gmailLabels := make(map[string]string)
	for _, l := range labelsRes.Labels {
		gmailLabels[strings.ToLower(l.Name)] = l.Name
	}

	// Scan and rewrite rules if casing is wrong compared to remote labels
	configRulesUpdated := false
	for i, rule := range config.Rules {
		parts := strings.Split(rule.Label, "/")
		rewritten := false
		for j := 1; j <= len(parts); j++ {
			prefix := strings.Join(parts[:j], "/")
			lowerPrefix := strings.ToLower(prefix)
			if exactName, ok := gmailLabels[lowerPrefix]; ok {
				exactParts := strings.Split(exactName, "/")
				for idx, val := range exactParts {
					if idx >= len(parts) {
						break
					}
					if parts[idx] != val {
						parts[idx] = val
						rewritten = true
					}
				}
			}
		}

		if rewritten {
			newLabel := strings.Join(parts, "/")
			if newLabel != rule.Label {
				fmt.Printf("%s Casing mismatch detected for rule '%s': Rewriting local config label %q to %q to match existing Gmail folders.\n",
					app.PrefixInfo, rule.Sender, rule.Label, newLabel)
				config.Rules[i].Label = newLabel
				configRulesUpdated = true
			}
		}
	}

	// Save back to config.json if there were changes
	if configRulesUpdated {
		fmt.Printf("%s Saving updated rule labels with corrected casing to config.json...\n", app.PrefixInfo)
		errSave := updateLocalRulesInConfig(config, config.Rules)
		if errSave != nil {
			fmt.Printf("%s Warning: Failed to save corrected rules to config.json: %v\n", app.PrefixWarn, errSave)
		} else {
			fmt.Printf("%s Successfully saved updated rules to config.json\n", app.PrefixSuccess)
		}
	}

	labelNameToID := make(map[string]string)
	labelIDToName := make(map[string]string)
	for _, l := range labelsRes.Labels {
		lowerName := strings.ToLower(l.Name)
		labelNameToID[lowerName] = l.Id
		labelIDToName[l.Id] = l.Name
	}

	fmt.Printf("%s Fetching existing filters from Gmail...\n", app.PrefixInfo)
	filtersRes, err := srv.Users.Settings.Filters.List("me").Do()
	if err != nil {
		return HandleScopeError(config, fmt.Errorf("failed to list Gmail filters: %w", err))
	}

	// Helper to find existing filter for a rule
	findExistingFilter := func(r Rule) *gmailapi.Filter {
		cleanSubject := strings.ToLower(strings.TrimSpace(r.Subject))
		cleanSender := strings.ToLower(strings.TrimSpace(r.Sender))

		for _, f := range filtersRes.Filter {
			if f.Criteria == nil {
				continue
			}
			matchSubject := true
			matchSender := true

			if cleanSubject != "" {
				matchSubject = strings.ToLower(strings.TrimSpace(f.Criteria.Subject)) == cleanSubject
			}
			if cleanSender != "" {
				matchSender = strings.ToLower(strings.TrimSpace(f.Criteria.From)) == cleanSender
			}

			if (cleanSubject != "" || cleanSender != "") && matchSubject && matchSender {
				return f
			}
		}
		return nil
	}

	fmt.Printf("%s Processing %d local rules...\n", app.PrefixInfo, len(config.Rules))
	createdCount := 0
	skippedCount := 0
	configRulesStatusChanged := false

	for i, rule := range config.Rules {
		if rule.Internal {
			continue
		}
		if rule.Sender == "" && rule.Subject == "" {
			continue
		}
		if rule.Label == "" {
			continue
		}

		ruleDesc := rule.Sender
		if rule.Subject != "" {
			ruleDesc = fmt.Sprintf("subject:%s", rule.Subject)
		}

		if rule.Exported && !config.ForceLearn {
			if config.Verbose {
				fmt.Printf("    %s Rule for '%s' -> label %q is already marked as exported. Skipping.\n", app.PrefixSuccess, ruleDesc, rule.Label)
			}
			skippedCount++
			continue
		}

		// 1. Ensure target label exists on Gmail
		labelID, errLabel := ensureLabelHierarchyREST(srv, rule.Label, labelNameToID, labelIDToName)
		if errLabel != nil {
			if IsInsufficientScopeError(errLabel) {
				return HandleScopeError(config, errLabel)
			}
			fmt.Printf("%s Warning: Failed to ensure label %q for rule '%s': %v\n", app.PrefixWarn, rule.Label, ruleDesc, errLabel)
			if config.Rules[i].Exported {
				config.Rules[i].Exported = false
				configRulesStatusChanged = true
			}
			continue
		}

		// 2. Check if a filter for this rule already exists
		existing := findExistingFilter(rule)
		if existing != nil {
			// Check if action matches target label
			hasAddLabel := false
			if existing.Action != nil {
				for _, addID := range existing.Action.AddLabelIds {
					if addID == labelID {
						hasAddLabel = true
						break
					}
				}
			}

			if hasAddLabel {
				if config.Verbose {
					fmt.Printf("    %s Rule for '%s' -> label %q already exists on Gmail. Skipping.\n", app.PrefixSuccess, ruleDesc, rule.Label)
				}
				if !config.Rules[i].Exported {
					config.Rules[i].Exported = true
					configRulesStatusChanged = true
				}
				skippedCount++
				continue
			}

			// If filter exists but has a different action
			if config.ForceLearn {
				fmt.Printf("%s Conflict found for '%s'. Force overwriting existing remote filter...\n", app.PrefixInfo, ruleDesc)
				errDel := srv.Users.Settings.Filters.Delete("me", existing.Id).Do()
				if errDel != nil {
					fmt.Printf("%s Warning: Failed to delete existing remote filter on Gmail: %v\n", app.PrefixWarn, errDel)
					if config.Rules[i].Exported {
						config.Rules[i].Exported = false
						configRulesStatusChanged = true
					}
				} else {
					// Create the new filter
					criteria := &gmailapi.FilterCriteria{}
					if rule.Subject != "" {
						criteria.Subject = rule.Subject
					}
					if rule.Sender != "" {
						criteria.From = rule.Sender
					}
					filter := &gmailapi.Filter{
						Criteria: criteria,
						Action: &gmailapi.FilterAction{
							AddLabelIds:    []string{labelID},
							RemoveLabelIds: []string{"INBOX"},
						},
					}
					_, errCreate := srv.Users.Settings.Filters.Create("me", filter).Do()
					if errCreate != nil {
						if IsInsufficientScopeError(errCreate) {
							return HandleScopeError(config, errCreate)
						}
						fmt.Printf("%s Warning: Failed to recreate filter for '%s' on Gmail: %v\n", app.PrefixWarn, ruleDesc, errCreate)
						if config.Rules[i].Exported {
							config.Rules[i].Exported = false
							configRulesStatusChanged = true
						}
					} else {
						fmt.Printf("%s Successfully overwrote Gmail server filter for '%s' -> label %q (archives automatically)\n",
							app.PrefixSuccess, ruleDesc, rule.Label)
						createdCount++
						if !config.Rules[i].Exported {
							config.Rules[i].Exported = true
							configRulesStatusChanged = true
						}
						continue
					}
				}
			}

			remoteActionDesc := describeFilterAction(existing.Action, labelIDToName)
			fmt.Printf("%s Filter for '%s' already exists on Gmail with different action (%s). Skipping to avoid conflict.\n",
				app.PrefixWarn, ruleDesc, remoteActionDesc)
			fmt.Printf("    -> Remote filter action: %s\n", remoteActionDesc)
			fmt.Printf("    -> Local rule suggests:  Add label %q and archive (remove INBOX)\n", rule.Label)

			fmt.Printf("    -> To overwrite conflicting remote filters, run 'mail_cli rule export force' or use '--force'.\n")
			if config.Rules[i].Exported {
				config.Rules[i].Exported = false
				configRulesStatusChanged = true
			}
			skippedCount++
			continue
		}

		// 3. Create the filter
		criteria := &gmailapi.FilterCriteria{}
		if rule.Subject != "" {
			criteria.Subject = rule.Subject
		}
		if rule.Sender != "" {
			criteria.From = rule.Sender
		}
		filter := &gmailapi.Filter{
			Criteria: criteria,
			Action: &gmailapi.FilterAction{
				AddLabelIds:    []string{labelID},
				RemoveLabelIds: []string{"INBOX"},
			},
		}

		_, errCreate := srv.Users.Settings.Filters.Create("me", filter).Do()
		if errCreate != nil {
			if IsInsufficientScopeError(errCreate) {
				return HandleScopeError(config, errCreate)
			}
			fmt.Printf("%s Warning: Failed to create filter for '%s' on Gmail: %v\n", app.PrefixWarn, ruleDesc, errCreate)
			if config.Rules[i].Exported {
				config.Rules[i].Exported = false
				configRulesStatusChanged = true
			}
			continue
		}

		fmt.Printf("%s Successfully created Gmail server filter for '%s' -> label %q (archives automatically)\n",
			app.PrefixSuccess, ruleDesc, rule.Label)
		createdCount++
		if !config.Rules[i].Exported {
			config.Rules[i].Exported = true
			configRulesStatusChanged = true
		}
	}

	// Save back to config.json if there were changes to Exported field
	if configRulesStatusChanged {
		fmt.Printf("%s Saving updated rule export statuses to config.json...\n", app.PrefixInfo)
		errSave := updateLocalRulesInConfig(config, config.Rules)
		if errSave != nil {
			fmt.Printf("%s Warning: Failed to save updated rules to config.json: %v\n", app.PrefixWarn, errSave)
		} else {
			fmt.Printf("%s Successfully saved updated rules to config.json\n", app.PrefixSuccess)
		}
	}

	fmt.Printf("\n%s Export finished. Created %d new filters, skipped %d rules.\n", app.PrefixSuccess, createdCount, skippedCount)
	return nil
}

func describeFilterAction(action *gmailapi.FilterAction, labelIDToName map[string]string) string {
	if action == nil {
		return "no action defined"
	}
	var parts []string

	if len(action.AddLabelIds) > 0 {
		var labels []string
		for _, id := range action.AddLabelIds {
			if name, ok := labelIDToName[id]; ok {
				labels = append(labels, name)
			} else {
				labels = append(labels, id)
			}
		}
		parts = append(parts, fmt.Sprintf("Add label(s): %s", strings.Join(labels, ", ")))
	}

	if len(action.RemoveLabelIds) > 0 {
		var removes []string
		for _, id := range action.RemoveLabelIds {
			if name, ok := labelIDToName[id]; ok {
				removes = append(removes, name)
			} else {
				removes = append(removes, id)
			}
		}
		parts = append(parts, fmt.Sprintf("Remove label(s): %s", strings.Join(removes, ", ")))
	}

	if action.Forward != "" {
		parts = append(parts, fmt.Sprintf("Forward to: %s", action.Forward))
	}

	if len(parts) == 0 {
		return "other action"
	}
	return strings.Join(parts, "; ")
}

// updateLocalRulesInConfig saves the rules slice back to ~/.config/mail_cli/config.json
