package cli

import (
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func scanQueuePodcasts(root string) []podcastDirEntry {
	var entries []podcastDirEntry
	for _, p := range podcast.ScanPodcastDirsReadOnly(root) {
		entries = append(entries, podcastDirEntry{dir: p.Dir, folderName: p.FolderName, title: p.Title, shortID: p.ShortID})
	}
	return entries
}

func resolveQueueTarget(root, query string) (*podcast.ResolvedID, error) {
	var matches []*podcast.ResolvedID
	for i, p := range scanQueuePodcasts(root) {
		if strings.EqualFold(query, p.shortID) || strings.EqualFold(query, p.title) || query == p.folderName || query == p.dir || query == strconv.Itoa(i+1) {
			return &podcast.ResolvedID{Type: podcast.ResolvedTypePodcast, Podcast: &podcast.ResolvedPodcast{Dir: p.dir, Title: p.title, ShortID: p.shortID, FolderName: p.folderName}}, nil
		}
		for _, path := range findMP3Files(p.dir) {
			if !pipeline.IsQueueAudioPath(path) {
				continue
			}
			id := podcast.EpisodeShortIDReadOnly(p.dir, p.shortID, path)
			title := episodeTitleFromPath(path)
			if strings.EqualFold(query, id) || query == path || query == filepath.Base(path) || query == title || query == queueFilenameForPath(p.dir, path) {
				matches = append(matches, &podcast.ResolvedID{Type: podcast.ResolvedTypeEpisode, Episode: &podcast.ResolvedEpisode{Path: path, Filename: filepath.Base(path), ShortID: id, Title: title, PodcastDir: p.dir, PodcastTitle: p.title, PodcastShortID: p.shortID}})
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, fmt.Errorf("queue target %q matches %d episodes; use a unique ID or path", query, len(matches))
}
