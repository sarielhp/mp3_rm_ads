package gmail

// gmailSystemLabels contains the Gmail system labels to include in the TUI folder tree.
var gmailSystemLabels = map[string]bool{
	"INBOX":   true,
	"SENT":    true,
	"TRASH":   true,
	"SPAM":    true,
	"DRAFT":   true,
	"STARRED": true,
}

// getLabelItemsGmailREST returns a []LabelItem for all Gmail labels suitable for the TUI.
func getLabelItemsGmailREST(config *Config) ([]LabelItem, error) {
	systemLabels, userLabels, err := fetchGmailLabelsREST(config)
	if err != nil {
		return nil, err
	}

	var items []LabelItem

	for _, l := range systemLabels {
		if !gmailSystemLabels[l.Name] {
			continue
		}
		items = append(items, LabelItem{
			Name:           l.Name,
			FullName:       l.Name,
			MessagesTotal:  l.MessagesTotal,
			MessagesUnread: l.MessagesUnread,
			IsLabel:        true,
		})
	}

	for _, l := range userLabels {
		items = append(items, LabelItem{
			Name:           l.Name,
			FullName:       l.Name,
			MessagesTotal:  l.MessagesTotal,
			MessagesUnread: l.MessagesUnread,
			IsLabel:        true,
		})
	}

	return items, nil
}
