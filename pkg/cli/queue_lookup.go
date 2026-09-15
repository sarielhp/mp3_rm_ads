package cli

import (
	"pod/pkg/podcast"
)

func scanQueuePodcasts(root string) []podcast.PodcastDirEntry {
	return podcast.ScanPodcastDirsReadOnly(root)
}

// resolveQueueTarget is for the handlers that still take a library root rather
// than a Library. It opens one for the lookup; the feed cache and download
// queue behind it are the process-wide instances, so this is cheap.
func resolveQueueTarget(root, query string) (*podcast.ResolvedID, error) {
	return podcast.Open(podcast.Config{PodcastsDir: root}, nil, nil).ResolveQueueTarget(query)
}
