package cli

import (
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"abs/pkg/util"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func scanQueuePodcasts(root string) []podcast.PodcastDirEntry {
	return podcast.ScanPodcastDirsReadOnly(root)
}

func resolveQueueTarget(root, query string) (*podcast.ResolvedID, error) {
	var matches []*podcast.ResolvedID
	for i, p := range scanQueuePodcasts(root) {
		if strings.EqualFold(query, p.ShortID) || strings.EqualFold(query, p.Title) || query == p.FolderName || query == p.Dir || query == strconv.Itoa(i+1) {
			return &podcast.ResolvedID{Type: podcast.ResolvedTypePodcast, Podcast: &podcast.ResolvedPodcast{Dir: p.Dir, Title: p.Title, ShortID: p.ShortID, FolderName: p.FolderName}}, nil
		}
		for _, path := range util.FindMP3Files(p.Dir) {
			if !pipeline.IsQueueAudioPath(path) {
				continue
			}
			id := podcast.EpisodeShortIDReadOnly(p.Dir, p.ShortID, path)
			title := podcast.EpisodeTitleFromPath(path)
			if strings.EqualFold(query, id) || query == path || query == filepath.Base(path) || query == title || query == queueFilenameForPath(p.Dir, path) {
				matches = append(matches, &podcast.ResolvedID{Type: podcast.ResolvedTypeEpisode, Episode: &podcast.ResolvedEpisode{Path: path, Filename: filepath.Base(path), ShortID: id, Title: title, PodcastDir: p.Dir, PodcastTitle: p.Title, PodcastShortID: p.ShortID}})
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, fmt.Errorf("queue target %q matches %d episodes; use a unique ID or path", query, len(matches))
}
