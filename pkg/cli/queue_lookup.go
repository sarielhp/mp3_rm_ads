package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"abs/pkg/util"
)

func scanQueuePodcasts(root string) []podcast.PodcastDirEntry {
	return podcast.ScanPodcastDirsReadOnly(root)
}

func resolveQueueTarget(root, query string) (*podcast.ResolvedID, error) {
	podcasts := scanQueuePodcasts(root)
	matchedPod, err := podcast.MatchLocalPodcasts(podcasts, query)
	if err != nil {
		if errors.Is(err, podcast.ErrAmbiguousPodcast) {
			return nil, err
		}
	} else if matchedPod != nil {
		return &podcast.ResolvedID{
			Type: podcast.ResolvedTypePodcast,
			Podcast: &podcast.ResolvedPodcast{
				Dir:        matchedPod.Dir,
				Title:      matchedPod.Title,
				ShortID:    matchedPod.ShortID,
				FolderName: matchedPod.FolderName,
			},
		}, nil
	}

	var matches []*podcast.ResolvedID
	for _, p := range podcasts {
		for _, path := range util.FindMP3Files(p.Dir) {
			if !pipeline.IsQueueAudioPath(path) {
				continue
			}
			id := podcast.EpisodeShortIDReadOnly(p.Dir, p.ShortID, path)
			title := podcast.EpisodeTitleFromPath(path)
			if strings.EqualFold(query, id) || query == path || query == filepath.Base(path) || query == title || query == queueFilenameForPath(p.Dir, path) {
				matches = append(matches, &podcast.ResolvedID{
					Type: podcast.ResolvedTypeEpisode,
					Episode: &podcast.ResolvedEpisode{
						Path:           path,
						Filename:       filepath.Base(path),
						ShortID:        id,
						Title:          title,
						PodcastDir:     p.Dir,
						PodcastTitle:   p.Title,
						PodcastShortID: p.ShortID,
					},
				})
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, fmt.Errorf("queue target %q matches %d episodes; use a unique ID or path", query, len(matches))
}
