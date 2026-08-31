package main

import (
	"fmt"
)

func selectEpisodesByDownloadPolicy(sortedCatalog []FeedEpisode, isDownloaded func(ep FeedEpisode) bool, policy string, k int, oldest bool) ([]FeedEpisode, []string) {
	normPolicy := normalizeDownloadPolicy(policy)
	if k <= 0 {
		k = 3
	}

	switch normPolicy {
	case DownloadPolicyNone:
		return nil, []string{"policy: none"}

	case DownloadPolicyLatest:
		if len(sortedCatalog) == 0 {
			return nil, nil
		}
		latestEp := sortedCatalog[len(sortedCatalog)-1]
		if oldest {
			latestEp = sortedCatalog[0]
		}
		hasEnc := (latestEp.Enclosure != nil && latestEp.Enclosure.URL != "") || latestEp.EnclosureURL != ""
		if hasEnc && !isDownloaded(latestEp) {
			return []FeedEpisode{latestEp}, []string{"1 latest episode (policy: latest)"}
		}
		return nil, nil

	case DownloadPolicyLatestK:
		if len(sortedCatalog) == 0 {
			return nil, nil
		}
		var toDownload []FeedEpisode
		if oldest {
			endIdx := min(len(sortedCatalog), k)
			for _, ep := range sortedCatalog[:endIdx] {
				hasEnc := (ep.Enclosure != nil && ep.Enclosure.URL != "") || ep.EnclosureURL != ""
				if hasEnc && !isDownloaded(ep) {
					toDownload = append(toDownload, ep)
				}
			}
		} else {
			startIdx := len(sortedCatalog) - k
			if startIdx < 0 {
				startIdx = 0
			}
			for _, ep := range sortedCatalog[startIdx:] {
				hasEnc := (ep.Enclosure != nil && ep.Enclosure.URL != "") || ep.EnclosureURL != ""
				if hasEnc && !isDownloaded(ep) {
					toDownload = append(toDownload, ep)
				}
			}
			for i, j := 0, len(toDownload)-1; i < j; i, j = i+1, j-1 {
				toDownload[i], toDownload[j] = toDownload[j], toDownload[i]
			}
		}
		if len(toDownload) == 0 {
			return nil, nil
		}
		return toDownload, []string{fmt.Sprintf("%d episode(s) (policy: latest_%d)", len(toDownload), k)}

	case DownloadPolicyAll:
		var undownloaded []FeedEpisode
		for _, ep := range sortedCatalog {
			hasEnc := (ep.Enclosure != nil && ep.Enclosure.URL != "") || ep.EnclosureURL != ""
			if hasEnc && !isDownloaded(ep) {
				undownloaded = append(undownloaded, ep)
			}
		}
		if len(undownloaded) == 0 {
			return nil, nil
		}
		if !oldest {
			for i, j := 0, len(undownloaded)-1; i < j; i, j = i+1, j-1 {
				undownloaded[i], undownloaded[j] = undownloaded[j], undownloaded[i]
			}
		}
		return undownloaded, []string{fmt.Sprintf("%d episode(s) (policy: all)", len(undownloaded))}

	default:
		return nil, nil
	}
}
