package gmail

import (
	"fmt"
	"strings"
	"sync"

	gmailapi "google.golang.org/api/gmail/v1"
)

// fetchGmailLabelsREST fetches all Gmail labels with populated message counts,
// split into system and user label lists.
func fetchGmailLabelsREST(config *Config) (systemLabels, userLabels []*gmailapi.Label, err error) {
	srv, err := GetGmailService(config)
	if err != nil {
		return nil, nil, err
	}

	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list Gmail labels: %w", err)
	}

	for _, l := range labelsRes.Labels {
		if strings.EqualFold(l.Type, "system") {
			systemLabels = append(systemLabels, l)
		} else {
			userLabels = append(userLabels, l)
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 15)
	var mu sync.Mutex

	fetchDetails := func(list []*gmailapi.Label) {
		for _, l := range list {
			wg.Add(1)
			go func(label *gmailapi.Label) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				detail, err := srv.Users.Labels.Get("me", label.Id).Do()
				if err == nil {
					mu.Lock()
					label.MessagesTotal = detail.MessagesTotal
					label.MessagesUnread = detail.MessagesUnread
					mu.Unlock()
				}
			}(l)
		}
	}

	fetchDetails(systemLabels)
	fetchDetails(userLabels)
	wg.Wait()

	var searchWg sync.WaitGroup
	searchSem := make(chan struct{}, 15)

	for _, l := range userLabels {
		if l.MessagesTotal == 0 {
			searchWg.Add(1)
			go func(label *gmailapi.Label) {
				searchSem <- struct{}{}
				defer func() { <-searchSem }()
				defer searchWg.Done()

				qTotal := fmt.Sprintf("label:%q", label.Name)
				resTotal, errTotal := srv.Users.Messages.List("me").Q(qTotal).IncludeSpamTrash(true).Do()
				if errTotal == nil {
					mu.Lock()
					label.MessagesTotal = int64(len(resTotal.Messages))
					if resTotal.NextPageToken != "" && resTotal.ResultSizeEstimate > label.MessagesTotal {
						label.MessagesTotal = resTotal.ResultSizeEstimate
					}
					mu.Unlock()
				}

				qUnread := fmt.Sprintf("label:%q is:unread", label.Name)
				resUnread, errUnread := srv.Users.Messages.List("me").Q(qUnread).IncludeSpamTrash(true).Do()
				if errUnread == nil {
					mu.Lock()
					label.MessagesUnread = int64(len(resUnread.Messages))
					if resUnread.NextPageToken != "" && resUnread.ResultSizeEstimate > label.MessagesUnread {
						label.MessagesUnread = resUnread.ResultSizeEstimate
					}
					mu.Unlock()
				}
			}(l)
		}
	}
	searchWg.Wait()

	sortLabels := func(list []*gmailapi.Label) {
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if strings.ToLower(list[i].Name) > strings.ToLower(list[j].Name) {
					list[i], list[j] = list[j], list[i]
				}
			}
		}
	}
	sortLabels(systemLabels)
	sortLabels(userLabels)

	return systemLabels, userLabels, nil
}
