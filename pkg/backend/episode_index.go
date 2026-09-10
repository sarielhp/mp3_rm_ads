package backend

// CatalogEpisode is a minimal episode identity taken from a server's catalog:
// just enough to tell whether an episode found in an RSS feed is already known,
// without carrying the full episode record.
type CatalogEpisode struct {
	PodcastID    string
	GUID         string
	EnclosureURL string
	Title        string
	Downloaded   bool
	AudioPath    string
	PublishedAt  int64
}

// CatalogIndexer is implemented by backends that can report every episode
// identity they hold in a single operation. Callers that only need to know
// which feed episodes are new should prefer it over fetching each podcast in
// turn, which costs a request per podcast and transfers the whole catalog.
type CatalogIndexer interface {
	CatalogEpisodes() ([]CatalogEpisode, error)
}

// PodcastLister is implemented by backends that can list podcast metadata
// without also transferring every episode of every podcast. Resolving a
// podcast by name, index or ID needs nothing more than this.
type PodcastLister interface {
	ListPodcasts() ([]Podcast, error)
}

// CatalogEpisodesFrom reports the catalog through CatalogIndexer when the
// backend supports it, and otherwise reports false so the caller can fall back
// to indexing the podcast records it already holds.
func CatalogEpisodesFrom(b Backend) ([]CatalogEpisode, bool) {
	indexer, ok := b.(CatalogIndexer)
	if !ok {
		return nil, false
	}
	eps, err := indexer.CatalogEpisodes()
	if err != nil || len(eps) == 0 {
		return nil, false
	}
	return eps, true
}

// ListPodcastsFrom lists podcast metadata through PodcastLister when the
// backend supports it, falling back to the full Podcasts() catalog fetch.
func ListPodcastsFrom(b Backend) ([]Podcast, error) {
	if lister, ok := b.(PodcastLister); ok {
		if pods, err := lister.ListPodcasts(); err == nil && len(pods) > 0 {
			return pods, nil
		}
	}
	return b.Podcasts()
}
